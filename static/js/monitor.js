/* Crawler Monitor — SSE 클라이언트 */

(function() {
  'use strict';

  var container = document.getElementById('event-log');
  if (!container) return;

  var agentID = container.dataset.agentId;
  if (!agentID) return;

  var sseStatus = document.getElementById('sse-status');

  function setStatus(text, cls) {
    if (!sseStatus) return;
    sseStatus.textContent = text;
    sseStatus.className = 'pull-right small ' + (cls || '');
  }

  function statusIcon(status) {
    switch (status) {
      case 'matched':        return '<i class="fa fa-magnifying-glass text-warning"></i>';
      case 'version_changed': return '<i class="fa fa-arrow-up text-primary"></i>';
      case 'error':          return '<i class="fa fa-exclamation-triangle text-danger"></i>';
      default:               return '<i class="fa fa-check text-muted"></i>';
    }
  }

  function rowClass(status) {
    switch (status) {
      case 'matched':        return 'event-row event-row-matched';
      case 'version_changed': return 'event-row event-row-version_changed';
      case 'error':          return 'event-row event-row-error';
      default:               return 'event-row event-row-checked';
    }
  }

  function formatTime(iso) {
    try {
      var d = new Date(iso);
      return d.toLocaleTimeString('ko-KR', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    } catch(e) {
      return iso;
    }
  }

  function appendEvent(ev) {
    // "이벤트 대기 중" 메시지 제거
    var noMsg = document.getElementById('no-events-msg');
    if (noMsg) noMsg.remove();

    var row = document.createElement('div');
    row.className = rowClass(ev.status);
    row.innerHTML =
      '<span class="event-time">' + formatTime(ev.occurred_at) + '</span>' +
      statusIcon(ev.status) +
      '<span class="event-message">' + escapeHTML(ev.message) + '</span>' +
      (ev.mode ? '<span class="label label-default event-mode">' + escapeHTML(ev.mode) + '</span>' : '');

    // 최신 이벤트를 맨 위에 삽입
    container.insertBefore(row, container.firstChild);

    // 최대 100건 유지
    var rows = container.querySelectorAll('.event-row');
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
    setStatus('연결 중...', '');
    var es = new EventSource('/agents/' + agentID + '/events');

    es.onopen = function() {
      setStatus('● 연결됨', 'connected');
    };

    es.onmessage = function(e) {
      try {
        var data = JSON.parse(e.data);
        if (data.type === 'connected') return; // 초기 핸드셰이크
        appendEvent(data);
      } catch(err) {
        console.warn('[SSE] parse error:', err);
      }
    };

    es.onerror = function() {
      setStatus('● 연결 끊김 (재연결 시도 중...)', 'disconnected');
      es.close();
      setTimeout(connect, 3000);
    };
  }

  connect();
})();
