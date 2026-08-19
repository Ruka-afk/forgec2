"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface MutationCallbacks<TArgs extends unknown[], TResult> {
  fn: (...args: TArgs) => Promise<TResult>;
  onSuccess?: (result: TResult, ...args: TArgs) => void;
  onError?: (error: unknown, ...args: TArgs) => void;
  onSettled?: (...args: TArgs) => void;
}

/**
 * Minimal async-mutation wrapper: tracks pending/error and guards against
 * stale completions when a newer mutation is fired while one is in flight.
 */
export function useMutation<TArgs extends unknown[], TResult>(
  callbacks: MutationCallbacks<TArgs, TResult>
): {
  mutate: (...args: TArgs) => Promise<TResult | undefined>;
  isPending: boolean;
  error: unknown;
  reset: () => void;
} {
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const callbacksRef = useRef(callbacks);
  useEffect(() => {
    callbacksRef.current = callbacks;
  });
  const generationRef = useRef(0);

  const mutate = useCallback(async (...args: TArgs) => {
    const gen = ++generationRef.current;
    setIsPending(true);
    setError(null);
    try {
      const result = await callbacksRef.current.fn(...args);
      if (gen === generationRef.current) callbacksRef.current.onSuccess?.(result, ...args);
      return result;
    } catch (err) {
      if (gen === generationRef.current) {
        setError(err);
        callbacksRef.current.onError?.(err, ...args);
      }
      return undefined;
    } finally {
      if (gen === generationRef.current) {
        setIsPending(false);
        callbacksRef.current.onSettled?.(...args);
      }
    }
  }, []);

  const reset = useCallback(() => {
    generationRef.current++;
    setError(null);
    setIsPending(false);
  }, []);

  return { mutate, isPending, error, reset };
}