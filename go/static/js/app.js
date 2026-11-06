/* fnUPnP 前端逻辑（零依赖原生 JS） */
(function () {
  "use strict";

  var state = {
    status: null,
    mappings: null,
    protocol: "TCP",
  };

  var $ = function (id) { return document.getElementById(id); };

  /* ---------------- 基础 API 封装 ---------------- */
  // 使用相对路径，使页面无论在端口直连（/）还是飞牛统一网关（/app/fnportgate/）
  // 下访问，都能正确命中同一个后端。
  function api(path, options) {
    var opts = options || {};
    opts.headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
    return fetch("api" + path, opts).then(function (res) {
      if (res.status === 204) return null;
      return res.json().catch(function () { return null; }).then(function (data) {
        if (!res.ok) {
          var msg = (data && data.detail) ? data.detail : ("请求失败（" + res.status + "）");
          throw new Error(msg);
        }
        return data;
      });
    });
  }

  function toast(message, type) {
    var c = $("toast-container");
    var el = document.createElement("div");
    el.className = "toast" + (type ? " " + type : "");
    var msg = document.createElement("span");
    msg.className = "t-msg";
    msg.textContent = message;
    var close = document.createElement("button");
    close.className = "t-close";
    close.innerHTML = "&times;";
    close.setAttribute("aria-label", "关闭");
    close.addEventListener("click", function () { el.remove(); });
    el.appendChild(msg);
    el.appendChild(close);
    c.appendChild(el);
    setTimeout(function () { el.remove(); }, 4200);
  }

  /* ---------------- 状态胶囊 ---------------- */
  function renderConnPill(connected, loading) {
    var pill = $("conn-pill");
    var text = $("conn-text");
    pill.className = "pill";
    if (loading) {
      pill.classList.add("pill-connecting");
      text.textContent = "发现中…";
    } else if (connected) {
      pill.classList.add("pill-online");
      text.textContent = "已连接";
    } else {
      pill.classList.add("pill-offline");
      text.textContent = "未连接";
    }
  }

  /* ---------------- 状态卡片 ---------------- */
  function renderStats(status, list) {
    $("stat-external").textContent = status.external_ip || "—";
    var gw = status.model_name || status.manufacturer || "";
    $("stat-gateway").textContent = gw || (status.gateway_ip || "—");
    $("stat-gateway-sub").textContent = status.gateway_ip ? ("网关 " + status.gateway_ip) : (status.error || "");
    $("stat-conn").textContent = status.connected ? "已连接" : "未连接";

    var total = list.mappings.length;
    var active = list.mappings.filter(function (m) { return m.active; }).length;
    $("stat-count").textContent = String(total);
    $("stat-count-sub").textContent = "生效 " + active + " · 未生效 " + (total - active);
    $("foot-lan").textContent = list.lan_ip || "—";
  }

  /* ---------------- 映射列表 ---------------- */
  // 页面由两块独立面板构成：
  //   1) 端口映射     —— 本应用添加（managed）
  //   2) 其他设备添加 —— 路由器上已存在、非本应用托管
  function renderMappings(list) {
    var area = $("mapping-area");
    var extPanel = $("ext-panel");
    var extArea = $("external-area");
    var mappings = list.mappings || [];

    if (!list.connected) {
      extPanel.classList.add("hidden");
      area.innerHTML =
        '<div class="state">' +
          '<span class="emoji">📡</span>' +
          "<h3>未连接到路由器</h3>" +
          "<p>" + escapeHtml(list.error || "无法通过 UPnP 发现路由器网关。") + "</p>" +
          '<button class="btn btn-primary" id="btn-retry">重新发现</button>' +
        "</div>";
      bindRetry();
      return;
    }

    var mine = mappings.filter(function (m) { return m.managed; });
    var others = mappings.filter(function (m) { return !m.managed; });

    // 本应用添加
    if (mine.length === 0) {
      area.innerHTML =
        '<div class="state">' +
          '<span class="emoji">🔌</span>' +
          "<h3>还没有端口映射</h3>" +
          "<p>点击右上角「添加映射」，把路由器外网端口转发到这台 NAS 的内部端口。</p>" +
          '<button class="btn btn-primary" id="btn-empty-add">添加映射</button>' +
        "</div>";
      var b = $("btn-empty-add");
      if (b) b.addEventListener("click", openModal);
    } else {
      area.innerHTML = renderTable(mine);
    }

    // 其他设备添加：独立面板
    $("ext-count").textContent = String(others.length);
    extArea.innerHTML = others.length === 0
      ? '<div class="hint-empty">' +
          '<span class="hi-icon" aria-hidden="true">🛰️</span>' +
          "<span>路由器上暂时没有其他设备创建的转发规则</span>" +
        "</div>"
      : renderTable(others);
    extPanel.classList.remove("hidden");

    bindRowActions();
  }

  function renderTable(list) {
    return (
      '<div class="table-wrap">' +
        "<table>" +
          "<thead><tr>" +
            "<th>描述</th><th>协议</th><th>外部端口</th><th>内部地址</th><th>状态</th><th></th>" +
          "</tr></thead>" +
          "<tbody>" + list.map(buildRow).join("") + "</tbody>" +
        "</table>" +
      "</div>"
    );
  }

  // 单行渲染，两块面板共用
  function buildRow(m) {
    var protoBadge = m.protocol === "UDP"
      ? '<span class="badge badge-udp">UDP</span>'
      : '<span class="badge badge-tcp">TCP</span>';

    // 来源已由分组区分，状态列只表达"是否生效"，避免信息重复
    var statusTag = m.active
      ? '<span class="status-tag status-active">生效中</span>'
      : '<span class="status-tag status-inactive">未生效</span>';

    var descHtml = escapeHtml(m.description || "—");

    var actions = "";
    if (m.managed) {
      if (!m.active) {
        actions += '<button class="mini-btn primary" data-act="enable" data-id="' + m.id + '">重新启用</button>';
      }
      actions += '<button class="mini-btn danger" data-act="delete" data-id="' + m.id + '">删除</button>';
    } else {
      actions += '<button class="mini-btn danger" data-act="delete-ext" data-ext="' + m.external_port + '" data-proto="' + m.protocol + '">删除</button>';
    }

    return (
      "<tr>" +
        '<td class="td-desc">' + descHtml + "</td>" +
        "<td>" + protoBadge + "</td>" +
        '<td class="port">' + m.external_port + "</td>" +
        '<td class="addr">' + escapeHtml(m.internal_client || "—") + " : " + m.internal_port + "</td>" +
        "<td>" + statusTag + "</td>" +
        '<td><div class="row-actions">' + actions + "</div></td>" +
      "</tr>"
    );
  }

  function escapeHtml(str) {
    return String(str).replace(/[&<>"']/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c];
    });
  }

  function bindRetry() {
    var b = $("btn-retry");
    if (b) b.addEventListener("click", function () { rediscover(); });
  }

  function bindRowActions() {
    Array.prototype.forEach.call(document.querySelectorAll("[data-act]"), function (btn) {
      btn.addEventListener("click", function () {
        var act = btn.getAttribute("data-act");
        if (act === "delete") {
          var id = btn.getAttribute("data-id");
          if (confirm("确认删除这条端口映射？")) {
            api("/mappings/" + id, { method: "DELETE" })
              .then(function () { toast("端口映射已删除"); loadAll(); })
              .catch(function (e) { toast(e.message, "error"); });
          }
        } else if (act === "enable") {
          var id2 = btn.getAttribute("data-id");
          btn.disabled = true;
          api("/mappings/" + id2 + "/enable", { method: "POST" })
            .then(function () { toast("端口映射已重新启用"); loadAll(); })
            .catch(function (e) { toast(e.message, "error"); btn.disabled = false; });
        } else if (act === "delete-ext") {
          var ext = btn.getAttribute("data-ext");
          var proto = btn.getAttribute("data-proto");
          if (confirm("确认删除路由器上的外部映射 " + ext + "/" + proto + "？")) {
            api("/mappings/port/" + ext + "/" + proto, { method: "DELETE" })
              .then(function () { toast("端口映射已删除"); loadAll(); })
              .catch(function (e) { toast(e.message, "error"); });
          }
        }
      });
    });
  }

  /* ---------------- 数据加载 ---------------- */
  function loadAll(force) {
    renderConnPill(state.status ? state.status.connected : false, true);
    var statusP = api("/status" + (force ? "?force=true" : ""));
    var listP = api("/mappings");
    Promise.all([statusP, listP]).then(function (results) {
      var status = results[0];
      var list = results[1];
      state.status = status;
      state.mappings = list;
      renderConnPill(status.connected, false);
      renderStats(status, list);
      renderMappings(list);
    }).catch(function (e) {
      renderConnPill(false, false);
      toast(e.message || "加载失败", "error");
    });
  }

  function rediscover() {
    renderConnPill(false, true);
    api("/discover", { method: "POST" })
      .then(function (status) {
        state.status = status;
        toast(status.connected ? "已重新发现路由器" : (status.error || "未发现路由器"), status.connected ? undefined : "warn");
        loadAll();
      })
      .catch(function (e) { toast(e.message, "error"); loadAll(); });
  }

  /* ---------------- 弹窗 ---------------- */
  var modalProtocol = "TCP";

  function openModal() {
    $("modal").classList.remove("hidden");
    $("modal-error").classList.add("hidden");
    $("f-desc").value = "";
    $("f-ext").value = "";
    $("f-int").value = "";
    $("f-client").value = "";
    $("f-lease").value = "0";
    setProtocol("TCP");
    if (state.mappings && state.mappings.lan_ip) {
      $("f-client").placeholder = state.mappings.lan_ip;
    }
    $("f-desc").focus();
  }

  function closeModal() {
    $("modal").classList.add("hidden");
  }

  function setProtocol(proto) {
    modalProtocol = proto;
    Array.prototype.forEach.call(document.querySelectorAll("#f-protocol .seg-item"), function (btn) {
      btn.classList.toggle("active", btn.getAttribute("data-value") === proto);
    });
  }

  function submitModal() {
    var desc = $("f-desc").value.trim();
    var ext = parseInt($("f-ext").value, 10);
    var intp = parseInt($("f-int").value, 10);
    var client = $("f-client").value.trim();
    var lease = parseInt($("f-lease").value, 10) || 0;

    if (!ext || ext < 1 || ext > 65535) { showModalError("请输入有效的外部端口（1-65535）"); return; }
    if (!intp || intp < 1 || intp > 65535) { showModalError("请输入有效的内部端口（1-65535）"); return; }

    var payload = {
      description: desc,
      protocol: modalProtocol,
      external_port: ext,
      internal_port: intp,
      internal_client: client || null,
      lease_time: lease,
    };

    var btn = $("btn-modal-submit");
    btn.disabled = true;
    api("/mappings", { method: "POST", body: JSON.stringify(payload) })
      .then(function () {
        closeModal();
        toast("端口映射添加成功");
        loadAll();
      })
      .catch(function (e) {
        showModalError(e.message);
        btn.disabled = false;
      });
  }

  function showModalError(msg) {
    var el = $("modal-error");
    el.textContent = msg;
    el.classList.remove("hidden");
  }

  /* ---------------- 事件绑定 ---------------- */
  function bindEvents() {
    $("btn-refresh").addEventListener("click", function () { rediscover(); });
    $("btn-add").addEventListener("click", openModal);
    $("btn-modal-close").addEventListener("click", closeModal);
    $("btn-modal-cancel").addEventListener("click", closeModal);
    $("btn-modal-submit").addEventListener("click", submitModal);

    $("modal").addEventListener("click", function (e) {
      if (e.target === $("modal")) closeModal();
    });

    Array.prototype.forEach.call(document.querySelectorAll("#f-protocol .seg-item"), function (btn) {
      btn.addEventListener("click", function () { setProtocol(btn.getAttribute("data-value")); });
    });

    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && !$("modal").classList.contains("hidden")) closeModal();
    });
  }

  /* ---------------- 启动 ---------------- */
  bindEvents();
  loadAll();
  setInterval(function () { loadAll(); }, 20000);
})();
