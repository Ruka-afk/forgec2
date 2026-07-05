"use client";

import { Component, createElement, ErrorInfo, ReactNode } from "react";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;
      return createElement("div", {
        className: "flex flex-col items-center justify-center min-h-[300px] p-8 text-center",
        children: [
          createElement("div", {
            key: "icon",
            className: "w-14 h-14 rounded-2xl bg-red-100 dark:bg-red-900/30 flex items-center justify-center mb-4",
            children: createElement("i", {
              className: "fa-solid fa-bug text-2xl text-red-500",
            }),
          }),
          createElement("h2", {
            key: "title",
            className: "text-lg font-semibold text-slate-900 dark:text-slate-100 mb-2",
            children: "Something went wrong",
          }),
          createElement("p", {
            key: "desc",
            className: "text-sm text-slate-500 dark:text-slate-400 mb-4 max-w-md",
            children: this.state.error?.message || "An unexpected error occurred.",
          }),
          createElement("button", {
            key: "btn",
            onClick: () => this.setState({ hasError: false, error: null }),
            className: "px-5 h-10 bg-indigo-600 hover:bg-indigo-700 text-white text-sm font-medium rounded-xl transition-colors",
            children: "Try Again",
          }),
        ],
      });
    }
    return this.props.children;
  }
}
