/* When in mobile layout, the anchor navigation for submenus
*  doesn't work due to fixed body height when menu is toggled.
*  This script intercepts clicks on links, toggles the menu off
*  and performs the anchor navigation. */

window.addEventListener("load", function () {
    let href = window.location.pathname;
    const hash = window.location.hash;
    if (hash !== "") {
        href = hash
    }
    const sidebar = document.querySelector('.sidebar .toctree');
    const selector = function (href) {
        return `a[href="${href}"]`
    };
    let element = sidebar.querySelector(selector(href));
    if (!element) {
        href = window.location.pathname;
        element = document.querySelector(selector(href));
    }
    if (element) {
        element.scrollIntoView({behavior: "smooth", block: "center", inline: "nearest"});
    }
    addNewDocsButton()
});

function addNewDocsButton() {
    let navigationBox = document.querySelector(".navigation-top");
    if (navigationBox) {
        let newDocsButton = document.createElement('a');
        newDocsButton.appendChild(document.createTextNode("Try New Docs"));
        newDocsButton.className = "btn";
        newDocsButton.title = "Try New Docs";
        newDocsButton.href = "https://new.docs.victoriametrics.com";
        let lastA = document.querySelector(".navigation-top > a");
        if (lastA) {
            lastA.parentNode.insertBefore(newDocsButton, lastA);
        }
    }
}

$(document).on("click", '.shift li.toc a', function (e) {
    let segments = this.href.split('#');
    if (segments.length < 2) {
        /* ignore links without anchor */
        return true;
    }

    e.preventDefault();
    $("#toggle").click();
    setTimeout(function () {
        location.hash = segments.pop();
    }, 1)
});

/* Clipboard-copy snippet from https://github.com/marcoaugustoandrade/jekyll-clipboardjs/blob/master/copy.js */
let codes = document.querySelectorAll('.with-copy .highlight > pre > code');
let countID = 0;
codes.forEach((code) => {

    code.setAttribute("id", "code" + countID);

    let btn = document.createElement('button');
    btn.innerHTML = "Copy";
    btn.className = "btn-copy";
    btn.setAttribute("data-clipboard-action", "copy");
    btn.setAttribute("data-clipboard-target", "#code" + countID);
    code.before(btn);
    countID++;
});

let clipboard = new ClipboardJS('.btn-copy');
