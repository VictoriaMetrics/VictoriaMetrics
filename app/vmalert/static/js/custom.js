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

function filter(){
    if($('#groups').is(':checked')){
        filterGroups();
    }else{
        filterRules();
    }
}

function filterGroups(){
    $(".group-heading").show()
    $(".rule-table").removeClass('show');
    $(".rule").show();
    
    if($("#filter").val().length === 0){
        $(".group-heading").show()
        return
    }

    $( ".group-heading" ).each(function() {
        var groupName = $(this).attr('data-group-name');
        var filter = $("#filter").val()

        if (groupName.indexOf(filter) < 0){
            let id = $(this).attr('data-bs-target')
            $("div[id='"+id+"'").removeClass('show');
            $(this).hide();
        }else{
            $(this).show();
        }
    });
}

function filterRules(){
    $(".group-heading").show()
    $(".rule-table").removeClass('show');
    $(".rule").show();

    if($("#filter").val().length === 0){
        return
    }

    $( ".rule" ).each(function() {
        var ruleName = $(this).attr("data-rule-name");
        var filter = $("#filter").val()
        let target = $(this).attr('data-bs-target')

        if (ruleName.indexOf(filter) < 0){
            $(this).hide();
        }else{
            $("div[id='rules-"+target+"'").addClass('show');
            $(this).show();
        }
    });

    $( ".rule-table" ).each(function() {
        let c = $( ".row", this ).filter(function() {
            return $(this).is(":visible")
        }).length;
        if (c === 0) {
            let target = $(this).attr('id')
            $("div[data-bs-target='"+target+"'").removeClass('show');
            $("div[data-bs-target='"+target+"'").hide()
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
