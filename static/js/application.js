/* Crawler Monitor — Application JS */

$(function() {
  // Bootstrap alert 자동 닫기 (5초 후)
  setTimeout(function() {
    $('.flash .alert').fadeOut(500);
  }, 5000);

  // 웹훅 토글: 숨겨진 체크박스 상태를 토글 버튼 시각화에 동기화
  $(document).on('change', '.webhook-toggle-input', function() {
    var $label = $('label.webhook-toggle[for="' + this.id + '"]');
    if (this.checked) {
      $label.addClass('is-on');
    } else {
      $label.removeClass('is-on');
    }
  });
});
