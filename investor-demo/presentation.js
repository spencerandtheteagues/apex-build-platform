const revealNodes = Array.from(document.querySelectorAll(".reveal"));
const navLinks = Array.from(document.querySelectorAll(".deck-nav a"));
const countNodes = Array.from(document.querySelectorAll("[data-count]"));

const revealObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        entry.target.classList.add("in-view");
      }
    }
  },
  { threshold: 0.16 }
);

revealNodes.forEach((node) => revealObserver.observe(node));

const sectionObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue;
      const id = entry.target.getAttribute("id");
      navLinks.forEach((link) => {
        link.classList.toggle("active", link.getAttribute("href") === `#${id}`);
      });
    }
  },
  {
    rootMargin: "-30% 0px -50% 0px",
    threshold: 0.1,
  }
);

document.querySelectorAll("section[id]").forEach((section) => sectionObserver.observe(section));

const countObserver = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue;
      const node = entry.target;
      const target = Number(node.getAttribute("data-count") || "0");
      const duration = 900;
      const start = performance.now();

      const tick = (now) => {
        const progress = Math.min((now - start) / duration, 1);
        const value = Math.round(target * (0.3 + 0.7 * progress));
        node.textContent = String(progress < 1 ? Math.max(0, value) : target);
        if (progress < 1) {
          requestAnimationFrame(tick);
        }
      };

      requestAnimationFrame(tick);
      countObserver.unobserve(node);
    }
  },
  { threshold: 0.6 }
);

countNodes.forEach((node) => countObserver.observe(node));
