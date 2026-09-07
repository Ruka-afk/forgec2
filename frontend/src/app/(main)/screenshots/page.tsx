import { Navigate } from "react-router-dom";

export default function ScreenshotsRedirect() {
  return <Navigate to="/loot?tab=screenshots" replace />;
}