(function () {
  function hideLoadingScreen() {
    var loadingScreen = document.getElementById('loading-screen')
    if (!loadingScreen) {
      return
    }

    loadingScreen.style.opacity = '0'
    loadingScreen.style.transition = 'opacity 0.5s ease-out'

    window.setTimeout(function () {
      loadingScreen.style.display = 'none'
    }, 500)
  }

  window.addEventListener('load', function () {
    window.setTimeout(hideLoadingScreen, 1000)
  })

  window.addEventListener('error', function (event) {
    console.error('APEX.BUILD Error:', event.error || event.message)
  })

  window.addEventListener('unhandledrejection', function (event) {
    console.error('APEX.BUILD Unhandled Promise Rejection:', event.reason)
  })
})()
