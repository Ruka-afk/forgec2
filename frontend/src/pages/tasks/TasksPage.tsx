import { Suspense, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

function TasksRedirectInner() {
  const navigate = useNavigate();
  const [sp] = useSearchParams();
  useEffect(() => {
    const q = new URLSearchParams(sp.toString());
    q.set("tab", "tasks");
    navigate(`/timeline?${q.toString()}`, { replace: true });
  }, [navigate, sp]);
  return null;
}

export default function TasksPage() {
  return (
    <Suspense fallback={null}>
      <TasksRedirectInner />
    </Suspense>
  );
}
