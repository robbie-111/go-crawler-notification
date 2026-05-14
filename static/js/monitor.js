/* Crawler Monitor — SSE 클라이언트 */

(function() {
  'use strict';

  var table = document.getElementById('event-log');
  if (!table) return;

  // agentID: <div id="logs" data-agent-id="..."> 또는 탭 링크에서 읽음
  var logsPane = document.getElementById('logs');
  var agentID = logsPane
    ? logsPane.dataset.agentId
    : null;
  if (!agentID) {
    var tabLink = document.querySelector(".agent-show #show-tabs a[href='#logs']");
    agentID = tabLink ? tabLink.dataset.agentId : null;
  }
  if (!agentID) return;

  // ── 유틸 ─────────────────────────────────────────────────────
  function timeAgo(iso) {
    try {
      var diff    = (Date.now() - new Date(iso).getTime()) / 1000;
      var minutes = diff / 60;
      var hours   = diff / 3600;
      var days    = diff / 86400;
      if (minutes < 1)   return 'less than a minute';
      if (minutes < 2)   return '1 minute';
      if (minutes < 45)  return Math.floor(minutes) + ' minutes';
      if (minutes < 90)  return 'about 1 hour';
      if (hours   < 24)  return 'about ' + Math.floor(hours) + ' hours';
      if (hours   < 42)  return '1 day';
      if (days    < 30)  return Math.floor(days) + ' days';
      if (days    < 45)  return 'about 1 month';
      if (days    < 365) return Math.floor(days / 30) + ' months';
      if (days    < 548) return 'about 1 year';
      return Math.floor(days / 365) + ' years';
    } catch(e) { return iso; }
  }

  function truncate(str, max) {
    if (!str) return '';
    return str.length > max ? str.slice(0, max) + '\u2026' : str;
  }

  function escapeHTML(str) {
    if (!str) return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  // ── SSE 이벤트 행 삽입 ────────────────────────────────────────
  // huginn-clone과 동일하게 thead/tbody 없이 table에 바로 <tr> 삽입
  function appendEvent(ev) {
    var noMsg = document.getElementById('no-events-msg');
    if (noMsg) noMsg.remove();

    var fullMsg  = ev.message || '';
    var shortMsg = escapeHTML(truncate(fullMsg, 200));
    var safeMsg  = fullMsg.replace(/"/g, '&quot;');

    var tr = document.createElement('tr');
    tr.className = 'event-row event-row-' + (ev.status || '');
    tr.innerHTML =
      '<td>' + shortMsg + '</td>' +
      '<td>' + escapeHTML(timeAgo(ev.occurred_at)) + ' ago</td>' +
      '<td>' +
        '<div class="btn-group btn-group-xs">' +
          '<a href="#" class="btn btn-default show-log-details"' +
             ' data-modal-title="Info"' +
             ' data-modal-content="' + safeMsg + '">' +
            'Details' +
          '</a>' +
        '</div>' +
      '</td>';

    // 헤더 <tr> 바로 다음에 삽입 (최신이 상단)
    var firstDataRow = table.querySelector('tr.event-row');
    if (firstDataRow) {
      table.insertBefore(tr, firstDataRow);
    } else {
      // 헤더 tr 다음에 삽입
      var headerRow = table.querySelector('tr');
      if (headerRow && headerRow.nextSibling) {
        table.insertBefore(tr, headerRow.nextSibling);
      } else {
        table.appendChild(tr);
      }
    }

    // 최대 100건 유지
    var rows = table.querySelectorAll('tr.event-row');
    if (rows.length > 100) {
      rows[rows.length - 1].remove();
    }
  }

  // ── SSE 연결 ──────────────────────────────────────────────────
  function connect() {
    var es = new EventSource('/agents/' + agentID + '/events');

    es.onmessage = function(e) {
      try {
        var data = JSON.parse(e.data);
        if (data.type === 'connected') return;
        appendEvent(data);
      } catch(err) {
        console.warn('[SSE] parse error:', err);
      }
    };

    es.onerror = function() {
      es.close();
      setTimeout(connect, 3000);
    };
  }

  connect();
})();
