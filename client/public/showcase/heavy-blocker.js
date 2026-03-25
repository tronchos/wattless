(function () {
  const started = performance.now();
  let accumulator = 0;

  while (performance.now() - started < 650) {
    accumulator += Math.sqrt(Math.random() * 1000);
  }

  window.__heavyBlocker = accumulator;
})();
