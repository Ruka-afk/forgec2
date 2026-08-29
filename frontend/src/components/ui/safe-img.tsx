"use client";

import { useCallback, useEffect, useState } from "react";

/**
 * Drop-in replacement for <img> that hides the element on load error
 * and restores it when the src changes (e.g. gallery navigation).
 *
 * Replaces the pervasive pattern:
 *   onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
 * which permanently hides the image even after src changes.
 */
export function SafeImg({
  onError,
  onLoad,
  ...props
}: React.ComponentProps<"img">) {
  const [hidden, setHidden] = useState(false);
  // Reset hidden state when src changes so lightbox navigation doesn't
  // leave the new image invisible after the old one errored.
  useEffect(() => { setHidden(false); }, [props.src]);

  const handleError = useCallback(
    (e: React.SyntheticEvent<HTMLImageElement, Event>) => {
      setHidden(true);
      onError?.(e);
    },
    [onError],
  );
  const handleLoad = useCallback(
    (e: React.SyntheticEvent<HTMLImageElement, Event>) => {
      setHidden(false);
      onLoad?.(e);
    },
    [onLoad],
  );

  return (
    <img
      {...props}
      onError={handleError}
      onLoad={handleLoad}
      style={{ ...props.style, display: hidden ? "none" : props.style?.display }}
    />
  );
}
