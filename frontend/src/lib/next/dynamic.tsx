import { Suspense, lazy, type ComponentType, type ReactNode } from "react";

type DynamicOptions = {
  ssr?: boolean;
  loading?: (() => ReactNode) | ReactNode;
};

function dynamic<T extends ComponentType<never>>(
  factory: () => Promise<{ default: T }>,
  options?: DynamicOptions
): T {
  const Lazy = lazy(factory as unknown as () => Promise<{ default: ComponentType<unknown> }>);
  const fallback: ReactNode =
    typeof options?.loading === "function" ? options.loading() : (options?.loading ?? null);
  const Wrapped = (props: Record<string, unknown>) => (
    <Suspense fallback={fallback}>
      <Lazy {...props} />
    </Suspense>
  );
  Wrapped.displayName = "Dynamic";
  return Wrapped as unknown as T;
}

export default dynamic;