// formalize-webcam-mic.mjs — promote legacy string-typed tasks to constants.
import fs from "node:fs";

let t = fs.readFileSync("pkg/protocol/tasks.go", "utf8");
if (!t.includes("TaskTypeWebcam")) {
  t = t.replace(
    '\tTaskTypeHelp             = "help"',
    '\tTaskTypeHelp             = "help"\n\tTaskTypeWebcam           = "webcam"\n\tTaskTypeMic              = "mic"',
  );
  t = t.replace("\t\tTaskTypeHelp,", "\t\tTaskTypeHelp,\n\t\tTaskTypeWebcam,\n\t\tTaskTypeMic,");
  fs.writeFileSync("pkg/protocol/tasks.go", t);
  console.log("tasks.go: webcam/mic constants added");
} else console.log("tasks.go: already present");

const dPath = "pkg/protocol/taskspec_data.go";
let d = fs.readFileSync(dPath, "utf8");
const before = d;
d = d.replace('{Type: "webcam",', "{Type: TaskTypeWebcam,").replace('{Type: "mic",', "{Type: TaskTypeMic,");
fs.writeFileSync(dPath, d);
console.log(before === d ? "data file: no change" : "data file: constants wired");
