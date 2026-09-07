import { Navigate } from "react-router-dom";

export default function WorkflowsRedirect() {
  return <Navigate to="/automation#tab=workflows" replace />;
}