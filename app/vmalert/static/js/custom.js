function actionAll(isCollapse) {
    document.querySelectorAll('.collapse').forEach((collapse) => {
        if (isCollapse) {
            collapse.classList.remove('show');
        } else {
            collapse.classList.add('show');
        }
    });
}

function groupForState(key) {
    if (key) {
        location.href = `?state=${key}`;
    } else {
        window.location = window.location.pathname;
    }
}

function showBySelector(selector) {
    if (!selector) {
        return
    }
    const control = document.querySelector(`${selector} [data-bs-target]`);
    if (!control) {
        return
    }
    let target = document.getElementById(control.getAttribute('data-bs-target').slice(1));
    if (target) {
        target.classList.add('show');
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

// search shows or hides groups&rules that satisfy the search phrase.
// case-insensitive, respects GET param `search`.
function search() {
    let searchBox = document.getElementById('search');
    if (!searchBox) {
        return;
    }
    const searchPhrase = searchBox.value.toLowerCase();

    filterRules(searchPhrase);

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

function matchText(search, item) {
    const text = item.innerText.toLowerCase();
    return text.indexOf(search) >= 0;
}

function filterRules(searchPhrase) {
    document.querySelectorAll('.vm-group').forEach((group) => {
        if (!searchPhrase) {
            group.classList.add('vm-found');
            return;
        }
        for (const item of group.querySelectorAll('.vm-group-search')) {
            if (matchText(searchPhrase, item)) {
                group.classList.add('vm-found');
                return;
            }
        }
        group.classList.remove('vm-found');
        for (const item of group.querySelectorAll('.vm-item')) {
            if (matchText(searchPhrase, item)) {
                item.classList.add('vm-found');
                continue;
            }
            if (Array.from(item.querySelectorAll('.label')).find(l => matchText(searchPhrase, l))) {
                item.classList.add('vm-found');
                continue;
            }
            item.classList.remove('vm-found');
        }
    });
}

document.addEventListener('DOMContentLoaded', () => {
    // update search element with value from URL, if any
    const searchPhrase = getParamURL('search')
    const searchBox = document.getElementById('search');
    if (searchBox) {
        searchBox.addEventListener('keyup', debounce(search, 500));
        searchBox.value = searchPhrase;
    }

    // apply filtering by search phrase
    search()

    showBySelector(window.location.hash);

    document.querySelectorAll('[data-bs-toggle="tooltip"]').forEach((tooltip) => {
        new bootstrap.Tooltip(tooltip);
    });
});
