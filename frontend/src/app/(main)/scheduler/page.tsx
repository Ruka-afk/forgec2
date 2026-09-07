import { Navigate } from "react-router-dom";

export default function SchedulerRedirect() {
  return <Navigate to="/automation#tab=scheduled" replace />;
}