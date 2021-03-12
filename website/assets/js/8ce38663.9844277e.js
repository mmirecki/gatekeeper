(window.webpackJsonp=window.webpackJsonp||[]).push([[11],{84:function(e,t,n){"use strict";n.r(t),n.d(t,"frontMatter",(function(){return s})),n.d(t,"metadata",(function(){return i})),n.d(t,"toc",(function(){return c})),n.d(t,"default",(function(){return l}));var a=n(3),r=n(7),o=(n(0),n(97)),s={id:"exempt-namespaces",title:"Exempting Namespaces"},i={unversionedId:"exempt-namespaces",id:"exempt-namespaces",isDocsHomePage:!1,title:"Exempting Namespaces",description:"Exempting Namespaces from Gatekeeper using config resource",source:"@site/docs/exempt-namespaces.md",slug:"/exempt-namespaces",permalink:"/gatekeeper/website/docs/exempt-namespaces",editUrl:"https://open-policy-agent.github.io/gatekeeper/website/docs/docs/exempt-namespaces.md",version:"current",sidebar:"docs",previous:{title:"Replicating Data",permalink:"/gatekeeper/website/docs/sync"},next:{title:"Policy Library",permalink:"/gatekeeper/website/docs/library"}},c=[{value:"Exempting Namespaces from Gatekeeper using config resource",id:"exempting-namespaces-from-gatekeeper-using-config-resource",children:[]},{value:"Exempting Namespaces from the Gatekeeper Admission Webhook using <code>--exempt-namespace</code> flag",id:"exempting-namespaces-from-the-gatekeeper-admission-webhook-using---exempt-namespace-flag",children:[]}],p={toc:c};function l(e){var t=e.components,n=Object(r.a)(e,["components"]);return Object(o.b)("wrapper",Object(a.a)({},p,n,{components:t,mdxType:"MDXLayout"}),Object(o.b)("h2",{id:"exempting-namespaces-from-gatekeeper-using-config-resource"},"Exempting Namespaces from Gatekeeper using config resource"),Object(o.b)("p",null,"The config resource can be used as follows to exclude namespaces from certain processes for all constraints in the cluster. To exclude namespaces at a constraint level, use ",Object(o.b)("inlineCode",{parentName:"p"},"excludedNamespaces")," in the ",Object(o.b)("a",{parentName:"p",href:"#constraints"},"constraint")," instead."),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre",className:"language-yaml"},'apiVersion: config.gatekeeper.sh/v1alpha1\nkind: Config\nmetadata:\n  name: config\n  namespace: "gatekeeper-system"\nspec:\n  match:\n    - excludedNamespaces: ["kube-system", "gatekeeper-system"]\n      processes: ["*"]\n    - excludedNamespaces: ["audit-excluded-ns"]\n      processes: ["audit"]\n    - excludedNamespaces: ["audit-webhook-sync-excluded-ns"]\n      processes: ["audit", "webhook", "sync"]\n...\n')),Object(o.b)("p",null,"Available processes:"),Object(o.b)("ul",null,Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"audit")," process exclusion will exclude resources from specified namespace(s) in audit results."),Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"webhook")," process exclusion will exclude resources from specified namespace(s) from the admission webhook."),Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"sync")," process exclusion will exclude resources from specified namespace(s) from being synced into OPA."),Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"*")," includes all current processes above and includes any future processes.")),Object(o.b)("h2",{id:"exempting-namespaces-from-the-gatekeeper-admission-webhook-using---exempt-namespace-flag"},"Exempting Namespaces from the Gatekeeper Admission Webhook using ",Object(o.b)("inlineCode",{parentName:"h2"},"--exempt-namespace")," flag"),Object(o.b)("p",null,"Note that the following only exempts resources from the admission webhook. They will still be audited. Editing individual constraints or ",Object(o.b)("a",{parentName:"p",href:"#exempting-namespaces-from-gatekeeper-using-config-resource"},"config resource")," is\nnecessary to exclude them from audit."),Object(o.b)("p",null,"If it becomes necessary to exempt a namespace from Gatekeeper webhook entirely (e.g. you want ",Object(o.b)("inlineCode",{parentName:"p"},"kube-system")," to bypass admission checks), here's how to do it:"),Object(o.b)("ol",null,Object(o.b)("li",{parentName:"ol"},Object(o.b)("p",{parentName:"li"},"Make sure the validating admission webhook configuration for Gatekeeper has the following namespace selector:"),Object(o.b)("pre",{parentName:"li"},Object(o.b)("code",{parentName:"pre",className:"language-yaml"},"  namespaceSelector:\n    matchExpressions:\n    - key: admission.gatekeeper.sh/ignore\n      operator: DoesNotExist\n")),Object(o.b)("p",{parentName:"li"},"the default Gatekeeper manifest should already have added this. The default name for the\nwebhook configuration is ",Object(o.b)("inlineCode",{parentName:"p"},"gatekeeper-validating-webhook-configuration")," and the default\nname for the webhook that needs the namespace selector is ",Object(o.b)("inlineCode",{parentName:"p"},"validation.gatekeeper.sh"))),Object(o.b)("li",{parentName:"ol"},Object(o.b)("p",{parentName:"li"},"Tell Gatekeeper it's okay for the namespace to be ignored by adding a flag to the pod:\n",Object(o.b)("inlineCode",{parentName:"p"},"--exempt-namespace=<NAMESPACE NAME>"),". This step is necessary because otherwise the\npermission to modify a namespace would be equivalent to the permission to exempt everything\nin that namespace from policy checks. This way a user must explicitly have permissions\nto configure the Gatekeeper pod before they can add exemptions.")),Object(o.b)("li",{parentName:"ol"},Object(o.b)("p",{parentName:"li"},"Add the ",Object(o.b)("inlineCode",{parentName:"p"},"admission.gatekeeper.sh/ignore")," label to the namespace. The value attached\nto the label is ignored, so it can be used to annotate the reason for the exemption."))))}l.isMDXComponent=!0},97:function(e,t,n){"use strict";n.d(t,"a",(function(){return m})),n.d(t,"b",(function(){return d}));var a=n(0),r=n.n(a);function o(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function s(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var a=Object.getOwnPropertySymbols(e);t&&(a=a.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,a)}return n}function i(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?s(Object(n),!0).forEach((function(t){o(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):s(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function c(e,t){if(null==e)return{};var n,a,r=function(e,t){if(null==e)return{};var n,a,r={},o=Object.keys(e);for(a=0;a<o.length;a++)n=o[a],t.indexOf(n)>=0||(r[n]=e[n]);return r}(e,t);if(Object.getOwnPropertySymbols){var o=Object.getOwnPropertySymbols(e);for(a=0;a<o.length;a++)n=o[a],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(r[n]=e[n])}return r}var p=r.a.createContext({}),l=function(e){var t=r.a.useContext(p),n=t;return e&&(n="function"==typeof e?e(t):i(i({},t),e)),n},m=function(e){var t=l(e.components);return r.a.createElement(p.Provider,{value:t},e.children)},u={inlineCode:"code",wrapper:function(e){var t=e.children;return r.a.createElement(r.a.Fragment,{},t)}},b=r.a.forwardRef((function(e,t){var n=e.components,a=e.mdxType,o=e.originalType,s=e.parentName,p=c(e,["components","mdxType","originalType","parentName"]),m=l(n),b=a,d=m["".concat(s,".").concat(b)]||m[b]||u[b]||o;return n?r.a.createElement(d,i(i({ref:t},p),{},{components:n})):r.a.createElement(d,i({ref:t},p))}));function d(e,t){var n=arguments,a=t&&t.mdxType;if("string"==typeof e||a){var o=n.length,s=new Array(o);s[0]=b;var i={};for(var c in t)hasOwnProperty.call(t,c)&&(i[c]=t[c]);i.originalType=e,i.mdxType="string"==typeof e?e:a,s[1]=i;for(var p=2;p<o;p++)s[p]=n[p];return r.a.createElement.apply(null,s)}return r.a.createElement.apply(null,n)}b.displayName="MDXCreateElement"}}]);