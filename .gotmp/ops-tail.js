(function () { if (!document.getElementById(MENU_ID)) injectMenuEntry(); }).observe(observerTarget, { childList: true, subtree: true });
        }

        if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot, { once: true });
        else boot();
      })();
    