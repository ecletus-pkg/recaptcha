function grecaptcha_execute() {
    grecaptcha.execute('6Lerpv8UAAAAAP-856keEZgvSPQnKCJlhwlzuZcN', {action: 'ttp://h.unapu.com/default/auth/password/login'})
        .then(function(token) {
            var el = document.getElementById("g-recaptcha__ttp:----h.unapu.com--default--auth--password--login");
            el.value = token;
        })
})
}
window.addEventListener("load", func(){
    grecaptcha_execute();
    setInterval(function(){grecaptcha_execute()}, 150000);
})