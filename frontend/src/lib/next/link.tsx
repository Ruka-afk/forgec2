import { forwardRef } from "react";
import { Link as RRLink } from "react-router-dom";
import { cn } from "@/lib/utils";

type LinkProps = Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, "href"> & {
  href: string;
  prefetch?: boolean;
  replace?: boolean;
};

export const Link = forwardRef<HTMLAnchorElement, LinkProps>(function Link(
  { href, className, children, prefetch, replace, ...rest },
  ref
) {
  void prefetch;
  const internal =
    typeof href === "string" && (href.startsWith("/") || href.startsWith("#") || href.startsWith("?"));
  const merged = cn(internal ? null : "text-primary underline underline-offset-4", className);
  if (!internal) {
    return (
      <a ref={ref} href={href} className={merged} target="_blank" rel="noopener noreferrer" {...rest}>
        {children}
      </a>
    );
  }
  return (
    <RRLink ref={ref} to={href} className={merged} replace={replace} {...rest}>
      {children}
    </RRLink>
  );
});

export default Link;
