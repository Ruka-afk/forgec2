import { Navigate } from "react-router-dom";

export default function DocsRedirect() {
  return <Navigate to="/settings#tab=about" replace />;
}