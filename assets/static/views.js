(function() {
  'use strict';

  function init() {
    const parseBtn = document.getElementById('parse-btn');
    const parseView = document.getElementById('parse-view');

    const viewBtns = [
      document.getElementById('overview-btn'),
      document.getElementById('headers-btn'),
      document.getElementById('vcllogtree-btn')
    ];

    const contentViews = [
      document.getElementById('overview-view'),
      document.getElementById('headers-view'),
      document.getElementById('vcllogtree-view')
    ].filter(Boolean);

    function updateUI() {
      // Remove views that are not active

      const anyViewActive = viewBtns.some(btn => btn.classList.contains('active'));

      if (anyViewActive) {
        parseView.classList.remove('active');
        parseBtn.classList.remove('active');

        contentViews.forEach((view, index) => {
          if (viewBtns[index].classList.contains('active')) {
            view.classList.add('active');
          } else {
            view.classList.remove('active');
          }
        });
      } else {
        parseView.classList.add('active');
        parseBtn.classList.add('active');
        contentViews.forEach(view => view.classList.remove('active'));
      }
    }

    // Parse button click
    parseBtn.addEventListener('click', () => {
      viewBtns.forEach(btn => btn.classList.remove('active'));
      updateUI();
    });

    // View button clicks
    viewBtns.forEach((btn, index) => {
      btn.addEventListener('click', () => {
        btn.classList.toggle('active');
        updateUI();
      });
    });

    updateUI();
  }

  // Run when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
