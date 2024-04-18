function expandAll() {
    $('.group-heading').each(function () {
        let style = $(this).attr("style")
        // display only elements that are currently visible
        if (style === "display: none;") {
            return
        }
        $(this).next().addClass('show')
    });
}

function collapseAll() {
    $('.collapse').removeClass('show');
}

function showByID(id) {
    if (!id) {
        return
    }
    let parent = $("#" + id).parent();
    if (!parent) {
        return
    }
    let target = $("#" + parent.attr("data-bs-target"));
    if (target.length > 0) {
        target.addClass('show');
    }
}

function toggleByID(id) {
    if (id) {
        let el = $("#" + id);
        if (el.length > 0) {
            el.click();
        }
    }
}

function debounce(func, delay) {
    let timer;
    return function (...args) {
        clearTimeout(timer);
        timer = setTimeout(() => {
            func.apply(this, args);
        }, delay);
    };
}

$('#search').on("keyup", debounce(search, 500));

// search shows or hides groups&rules that satisfy the search phrase.
// case-insensitive, respects GET param `search`.
function search() {
    $(".rule").show();

    let groupHeader = $(".group-heading")
    let searchPhrase = $("#search").val().toLowerCase()
    if (searchPhrase.length === 0) {
        groupHeader.show()
        setParamURL('search', '')
        return
    }

    $(".rule-table").removeClass('show');
    groupHeader.hide()

    searchPhrase = searchPhrase.toLowerCase()
    filterRuleByName(searchPhrase);
    filterRuleByLabels(searchPhrase);
    filterGroupsByName(searchPhrase);

    setParamURL('search', searchPhrase)
}

function setParamURL(key, value) {
    let url = new URL(location.href)
    url.searchParams.set(key, value);
    window.history.replaceState(null, null, `?${url.searchParams.toString()}${url.hash}`);
}

function getParamURL(key) {
    let url = new URL(location.href)
    return url.searchParams.get(key)
}

function filterGroupsByName(searchPhrase) {
    $(".group-heading").each(function () {
        const groupName = $(this).attr('data-group-name').toLowerCase();
        const hasValue = groupName.indexOf(searchPhrase) >= 0

        if (!hasValue) {
            return
        }

        const target = $(this).attr("data-bs-target");
        $(`div[id="${target}"] .rule`).show();
        $(this).show();
    });
}

function filterRuleByName(searchPhrase) {
    $(".rule").each(function () {
        const ruleName = $(this).attr("data-rule-name").toLowerCase();
        const hasValue = ruleName.indexOf(searchPhrase) >= 0
        if (!hasValue) {
            $(this).hide();
            return
        }

        const target = $(this).attr('data-bs-target')
        $(`#rules-${target}`).addClass('show');
        $(`div[data-bs-target='rules-${target}']`).show();
        $(this).show();
    });
}

function filterRuleByLabels(searchPhrase) {
    $(".rule").each(function () {
        const matches = $(".label", this).filter(function () {
            const label = $(this).text().toLowerCase();
            return label.indexOf(searchPhrase) >= 0;
        }).length;

        if (matches > 0) {
            const target = $(this).attr('data-bs-target')
            $(`#rules-${target}`).addClass('show');
            $(`div[data-bs-target='rules-${target}']`).show();
            $(this).show();
        }
    });
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

    // update search element with value from URL, if any
    let searchPhrase = getParamURL('search')
    $("#search").val(searchPhrase)

    // apply filtering by search phrase
    search()

    let hash = window.location.hash.substr(1);
    showByID(hash);
});

$(document).ready(function () {
    $('[data-bs-toggle="tooltip"]').tooltip();
});
