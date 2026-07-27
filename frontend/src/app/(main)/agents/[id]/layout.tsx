import AgentDetailShell from "./_components/AgentDetailShell";

export { agentStaticParams as generateStaticParams } from "@/lib/constants";

export default function AgentDetailLayout({ children }: { children: React.ReactNode }) {
  return <AgentDetailShell>{children}</AgentDetailShell>;
}
