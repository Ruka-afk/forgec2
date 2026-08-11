import { redirect } from "next/navigation";

export default function SchedulerRedirect() {
  redirect("/automation#tab=scheduled");
}