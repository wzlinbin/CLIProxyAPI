}function g_e(t,e){let n="",i=t[e+1];for(;(i===" "||i==="	"||i===`
`||i==="\r")&&!(i==="\r"&&t[e+2]!==`
`);)i===`
`&&(n+=`
`),e+=1,i=t[e+1];return n||(n=" "),{fold:n,offset:e}}const __e={0:"\0",a:"\x07",b:"\b",e:"\x1B",f:"\f",n:`
`,r:"\r",t:"	",v:"\v",N:"\x85",_:" ",L:"\u2028",P:"\u2029"," ":" ",'"':'"',"/":"/","\\":"\\","	":"	"};function y_e(t,e,n,i){const s=t.substr(e,n),o=s.length===n&&/^[0-9a-fA-F]+$/.test(s