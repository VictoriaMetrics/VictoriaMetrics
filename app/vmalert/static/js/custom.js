function actionAll(isCollapse) {
    document.querySelectorAll('.collapse').forEach((collapse) => {
        if (isCollapse) {
            collapse.classList.remove('show');
        } else {
            collapse.classList.add('show');
        }
    });
}

function groupFilter(key) {
    if (key) {
        location.href = `?filter=${key}`;
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

function filterRules(searchPhrase) {
    document.querySelectorAll('.alert-rules').forEach((rules) => {
        let found = false;
        rules.querySelectorAll('.alert-rule').forEach((rule) => {
            if (searchPhrase) {
                const ruleName = rule.innerText.toLowerCase();
                const matches = []
                const hasValue = ruleName.indexOf(searchPhrase) >= 0;
                rule.querySelectorAll('.label').forEach((label) => {
                    const text = label.innerText.toLowerCase();
                    if (text.indexOf(searchPhrase) >= 0) {
                        matches.push(text);
                    }
                });
                if (!matches.length && !hasValue) {
                    rule.classList.add('d-none');
                    return;
                }
            }
            rule.classList.remove('d-none');
            found = true;
        });
        if (found && searchPhrase) {
            rules.classList.add('show');
        } else {
            rules.classList.remove('show');
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
