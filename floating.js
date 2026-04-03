(function () {
  function init() {
    const floatingBall = document.getElementById('desktop-floating-ball');

    if (!floatingBall) {
      return;
    }

    floatingBall.addEventListener('click', function () {
      if (window.CialloDesktop && typeof window.CialloDesktop.toggleMainPanel === 'function') {
        window.CialloDesktop.toggleMainPanel();
      }
    });
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
