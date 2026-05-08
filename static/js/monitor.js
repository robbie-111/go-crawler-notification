/* Crawler Monitor — SSE 클라이언트 */

(function() {
  'use strict';

  var table = document.getElementById('event-log');
  if (!table) return;

  var logsPane = document.getElementById('logs');
  var agentID = logsPane ? logsPane.dataset.agentId : null;
  if (!agentID) return;

  function statusClass(status) {
    switch (status) {
      case 'matched':         return 'warning';
      case 'version_changed': return 'primary';
      case 'error':           return 'danger';
      default:                return 'default';
    }
  }

  function formatTime(iso) {
    try {
      var d = new Date(iso);
      var pad = function(n) { return n < 10 ? '0' + n : n; };
      return d.getFullYear() + '-' +
             pad(d.getMonth() + 1) + '-' +
             pad(d.getDate()) + ' ' +
             pad(d.getHours()) + ':' +
             pad(d.getMinutes()) + ':' +
             pad(d.getSeconds());
    } catch(e) {
      return iso;
    }
  }

  function truncate(str, max) {
    if (!str) return '';
    return str.length > max ? str.slice(0, max) + '…' : str;
  }

  function appendEvent(ev) {
    // "이벤트 대기 중" 행 제거
    var noMsg = document.getElementById('no-events-msg');
    if (noMsg) noMsg.remove();

    var tbody = table.querySelector('tbody');
    if (!tbody) return;

    var tr = document.createElement('tr');
    tr.className = 'event-row event-row-' + ev.status;
    tr.innerHTML =
      '<td>' + escapeHTML(truncate(ev.message, 200)) + '</td>' +
      '<td>' + escapeHTML(formatTime(ev.occurred_at)) + '</td>' +
      '<td>' +
        '<span class="label label-' + statusClass(ev.status) + '">' + escapeHTML(ev.status) + '</span>' +
        (ev.mode ? ' <span class="label label-default event-mode">' + escapeHTML(ev.mode) + '</span>' : '') +
      '</td>';

    // 최신 이벤트를 tbody 맨 위에 삽입
    tbody.insertBefore(tr, tbody.firstChild);

    // 최대 100건 유지
    var rows = tbody.querySelectorAll('tr.event-row');
    if (rows.length > 100) {
      rows[rows.length - 1].remove();
    }
  }

  function escapeHTML(str) {
    if (!str) return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

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
