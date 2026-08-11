import { redirect } from "next/navigation";

export default function ScreenshotsRedirect() {
  redirect("/loot?tab=screenshots");
}