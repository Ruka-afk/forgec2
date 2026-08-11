import { redirect } from "next/navigation";

export default function WorkflowsRedirect() {
  redirect("/automation#tab=workflows");
}