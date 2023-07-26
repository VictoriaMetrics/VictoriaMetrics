function expandAll() {
    $('.collapse').addClass('show');
}

function collapseAll() {
    $('.collapse').removeClass('show');
}

function toggleByID(id) {
    if (id) {
        let el = $("#" + id);
        if (el.length > 0) {
            el.click();
        }
    }
}

$(document).ready(function () {
    $(".group-heading a").click(function (e) {
        e.stopPropagation(); // prevent collapse logic on link click
        let target = $(this).attr('href');
        if (target.length > 0) {
            toggleByID(target.substr(1));
        }
    });

    $(".group-heading").click(function (e) {
        let target = $(this).attr('data-bs-target');
        let el = $("#" + target);
        new bootstrap.Collapse(el, {
            toggle: true
        });
    });

    let hash = window.location.hash.substr(1);
    toggleByID(hash);
});

$(document).ready(function () {
    $('[data-bs-toggle="tooltip"]').tooltip();
});
