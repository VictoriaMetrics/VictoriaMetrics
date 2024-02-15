function expandAll() {
    $(".group-heading").show()
    $('.collapse').addClass('show');
}

function collapseAll() {
    $(".group-heading").show()
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

function debounce(func, delay) {
    let timer;
    return function(...args) {
        clearTimeout(timer);
        timer = setTimeout(() => {
            func.apply(this, args);
        }, delay);
    };
}

$('#filter').on("keyup", debounce(filter, 500));

function filter(){
    $(".rule-table").removeClass('show');
    $(".rule").show();
    
    if($("#filter").val().length === 0){
        $(".group-heading").show()
        return
    }

    $(".group-heading").hide()

    filterRuleByName();
    filterRuleByLabels();
    filterGroupsByName();
}

function filterGroupsByName(){
    $( ".group-heading" ).each(function() {
        const groupName = $(this).attr('data-group-name');
        const filter = $("#filter").val()
        const hasValue = groupName.indexOf(filter) >= 0

        if (hasValue){
            const target = $(this).attr("data-bs-target");
            
            $(this).show();
            $(`div[id="${target}"] .rule`).show();
        }
    });
}

function filterRuleByName(){
    $( ".rule" ).each(function() {
        const ruleName = $(this).attr("data-rule-name");
        const filter = $("#filter").val()
        const hasValue = ruleName.indexOf(filter) >= 0

        if (hasValue){
            const target = $(this).attr('data-bs-target')

            $(`#rules-${target}`).addClass('show');
            $(`div[data-bs-target='rules-${target}']`).show();
            $(this).show();
        }else{
            $(this).hide();
        }
    });  
}

function filterRuleByLabels(){
    $( ".rule" ).each(function() {
        const filter = $("#filter").val()
        
        const matches = $( ".label", this ).filter(function() {
            const label = $(this).text();
            const hasValue = label.indexOf(filter) >= 0
            return hasValue;
        }).length; 

        if (matches > 0){
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

    let hash = window.location.hash.substr(1);
    toggleByID(hash);
});

$(document).ready(function () {
    $('[data-bs-toggle="tooltip"]').tooltip();
});
