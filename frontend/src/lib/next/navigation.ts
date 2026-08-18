import { useLocation, useNavigate, useParams as useRRParams, useSearchParams as useRRSearchParams } from "react-router-dom";

type ParamsValue<T> = T extends string ? Readonly<Record<T, string>> : Readonly<T>;

export function useParams<
  ParamsOrKey extends string | Record<string, string | undefined> = string
>(): ParamsValue<ParamsOrKey> {
  return useRRParams() as ParamsValue<ParamsOrKey>;
}

export function redirect(to: string): never {
  window.location.assign(to);
  throw new Error(`redirect to ${to}`);
}

export function usePathname(): string {
  return useLocation().pathname;
}

export function useSearchParams(): URLSearchParams {
  const [params] = useRRSearchParams();
  return params;
}

export function useRouter() {
  const navigate = useNavigate();
  return {
    push: (to: string | number) => navigate(to as never),
    replace: (to: string | number) => navigate(to as never, { replace: true }),
    refresh: () => window.location.reload(),
    prefetch: () => {},
  };
}
