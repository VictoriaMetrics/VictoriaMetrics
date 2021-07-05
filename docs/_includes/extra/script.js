/* When in mobile layout, the anchor navigation for submenus
*  doesn't work due to fixed body height when menu is toggled.
*  This script intercepts clicks on links, toggles the menu off
*  and performs the anchor navigation. */
$(document).on("click", '.shift li.toc a', function(e) {
    let segments = this.href.split('#');
    if (segments.length < 2) {
        /* ignore links without anchor */
        return true;
    }

    e.preventDefault();
    $("#toggle").click();
    setTimeout(function () {
       location.hash = segments.pop();
    },1)
});
