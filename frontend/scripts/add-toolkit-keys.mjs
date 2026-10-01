// One-off: insert toolkit spray/container keys into zh.ts next to their
// siblings, mirroring the en.ts insertions.
import { readFileSync, writeFileSync } from "node:fs";

const file = "src/lib/i18n/zh.ts";
const lines = readFileSync(file, "utf8").split("\n");
const out = [];
for (const line of lines) {
  out.push(line);
  if (line.includes('"toolkit.tab_templates"')) {
    out.push('    "toolkit.tab_spray": "密码喷洒",');
  } else if (line.includes('"toolkit.cat_lateral_persist"')) {
    out.push('    "toolkit.cat_container": "容器",');
  } else if (line.includes('"toolkit.desc_usb_drop"')) {
    out.push('    "toolkit.desc_container_detect": "检测容器/虚拟化执行环境",');
    out.push('    "toolkit.desc_container_escape": "尝试通用容器逃逸",');
    out.push('    "toolkit.desc_container_docker": "尝试 Docker 容器逃逸",');
    out.push('    "toolkit.desc_container_k8s": "尝试 Kubernetes Pod 逃逸",');
  }
}
writeFileSync(file, out.join("\n"), "utf8");
console.log("zh toolkit keys added");
