/* Crawler Monitor — Application JS */

// ── 동적 모달 (huginn-clone Utils.showDynamicModal 동일 패턴) ────────────────
function showDynamicModal(message, title) {
  var existing = document.getElementById('dynamic-modal');
  if (existing) existing.remove();

  $('body').append(
    '<div class="modal fade" tabindex="-1" id="dynamic-modal" role="dialog"' +
        ' aria-labelledby="dynamic-modal-label" aria-hidden="true">' +
      '<div class="modal-dialog big-modal-dialog">' +
        '<div class="modal-content">' +
          '<div class="modal-header">' +
            '<button type="button" class="close" data-dismiss="modal">' +
              '<span aria-hidden="true">&times;</span>' +
              '<span class="sr-only">Close</span>' +
            '</button>' +
            '<h4 class="modal-title" id="dynamic-modal-label"></h4>' +
          '</div>' +
          '<div class="modal-body"><pre style="white-space:pre-wrap;"></pre></div>' +
        '</div>' +
      '</div>' +
    '</div>'
  );

  var $modal = $('#dynamic-modal');
  $modal.find('.modal-title').text(title || 'Info');
  $modal.find('.modal-body pre').text(message || '');
  $modal.on('hidden.bs.modal', function() {
    $('#dynamic-modal').remove();
  });
  $modal.modal('show');
}

// ── Logs 탭 AJAX 로드 (huginn-clone agent-show-page.js fetchLogs 동일 패턴) ──
function fetchLogs(agentId) {
  $('#logs .spinner').show();
  $('#logs .refresh, #logs .clear').hide();

  $.get('/agents/' + agentId + '/logs', function(html) {
    $('#logs .logs').html(html);

    $('#logs .spinner').stop(true, true).fadeOut(function() {
      $('#logs .refresh, #logs .clear').show();
    });
  });
}

$(function() {
  // ── Flash 자동 닫기 (5초) ──────────────────────────────────────────────────
  var $flash = $('.flash');
  if ($flash.length) {
    setTimeout(function() { $flash.slideUp(500); }, 5000);
  }

  // ── 웹훅 토글 ──────────────────────────────────────────────────────────────
  $(document).on('change', '.webhook-toggle-input', function() {
    var $label = $('label.webhook-toggle[for="' + this.id + '"]');
    if (this.checked) { $label.addClass('is-on'); }
    else              { $label.removeClass('is-on'); }
  });

  // ── Agent Show 페이지 로직 (/agents/{id}) ──────────────────────────────────
  if (window.location.pathname.match(/^\/agents\/[^/]+$/)) {

    // Logs 탭 클릭 → Bootstrap 탭 전환 + fetchLogs
    $(document).on('click', ".agent-show #show-tabs a[href='#logs']", function(e) {
      e.preventDefault();
      var agentId = $(e.currentTarget).closest('[data-agent-id]').data('agent-id')
                 || $('#logs').data('agent-id');
      $(e.currentTarget).tab('show');
      fetchLogs(agentId);
    });

    // refresh 버튼 클릭 → fetchLogs
    $(document).on('click', '#logs .refresh', function(e) {
      e.preventDefault();
      fetchLogs($('#logs').data('agent-id'));
    });

    // Details 버튼 클릭 (초기 서버사이드 렌더링 행 — AJAX 재로드 전)
    $(document).on('click', '.show-log-details', function(e) {
      e.preventDefault();
      var $btn = $(this);
      showDynamicModal($btn.data('modal-content'), $btn.data('modal-title'));
    });

    // Clear 버튼 클릭 → 해당 에이전트 로그 삭제 (huginn-clone clearLogs 패턴)
    $(document).on('click', '#logs .clear', function(e) {
      e.preventDefault();
      if (!confirm('이 에이전트의 로그를 모두 삭제할까요?')) return;
      var agentId = $('#logs').data('agent-id');
      $('#logs .spinner').show();
      $('#logs .refresh, #logs .clear').hide();
      $.post('/agents/' + agentId + '/logs/clear', function(html) {
        $('#logs .logs').html(html);
        $('#logs .spinner').stop(true, true).fadeOut(function() {
          $('#logs .refresh, #logs .clear').show();
        });
      });
    });
  }
});
