(function () {
  function hideLoadingScreen() {
    var loadingScreen = document.getElementById('loading-screen')
    if (!loadingScreen) {
      return
    }

    document.documentElement.classList.add('app-loaded')
    loadingScreen.style.opacity = '0'
    loadingScreen.style.transition = 'opacity 0.5s ease-out'

    window.setTimeout(function () {
      loadingScreen.style.display = 'none'
    }, 500)
  }

  function scheduleLoadingScreenHide(delay) {
    window.setTimeout(hideLoadingScreen, delay)
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () {
      scheduleLoadingScreenHide(450)
    })
  } else {
    scheduleLoadingScreenHide(450)
  }

  window.addEventListener('load', function () {
    scheduleLoadingScreenHide(250)
  })

  scheduleLoadingScreenHide(2500)

  window.addEventListener('error', function (event) {
    console.error('APEX-BUILD Error:', event.error || event.message)
  })

  window.addEventListener('unhandledrejection', function (event) {
    console.error('APEX-BUILD Unhandled Promise Rejection:', event.reason)
  })
})()
