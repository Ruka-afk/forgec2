// add-help-spec.mjs — one-shot: register the "help" builtin as a proper
// TaskSpec (constant + AllTaskTypes entry + data-file constant reference).
import fs from "node:fs";

let t = fs.readFileSync("pkg/protocol/tasks.go", "utf8");
if (!t.includes("TaskTypeHelp")) {
  t = t.replace(
    '\tTaskTypeHostInfo         = "hostinfo"',
    '\tTaskTypeHostInfo         = "hostinfo"\n\tTaskTypeHelp             = "help"',
  );
  t = t.replace("\t\tTaskTypeHostInfo,", "\t\tTaskTypeHostInfo,\n\t\tTaskTypeHelp,");
  fs.writeFileSync("pkg/protocol/tasks.go", t);
  console.log("tasks.go: TaskTypeHelp added");
} else {
  console.log("tasks.go: already present");
}

const dPath = "pkg/protocol/taskspec_data.go";
let d = fs.readFileSync(dPath, "utf8");
const before = d;
d = d.replace('{Type: "help", Name: "Help",', "{Type: TaskTypeHelp, Name: \"Help\",");
fs.writeFileSync(dPath, d);
console.log(before === d ? "data file: no change" : "data file: help spec now uses constant");
