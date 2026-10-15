function r(e){let t=e==null?"":String(e);return t=t.replace(/\r/g,""),/^[=+\-@\t]/.test(t)&&(t=`'${t}`),`"${t.replace(/"/g,'""')}"`}export{r as c};
