import { redirect } from "next/navigation";

export default function SchedulerRedirect() {
  redirect("/automation#scheduled");
}