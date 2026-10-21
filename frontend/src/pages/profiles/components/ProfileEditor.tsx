import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from "react";
import { EditorState } from "@codemirror/state";
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLineGutter,
  highlightActiveLine,
  drawSelection,
  dropCursor,
  rectangularSelection,
  crosshairCursor,
  placeholder,
} from "@codemirror/view";
import {
  defaultKeymap,
  history,
  historyKeymap,
  indentWithTab,
} from "@codemirror/commands";
import {
  bracketMatching,
  foldGutter,
  foldKeymap,
  indentOnInput,
} from "@codemirror/language";
import { searchKeymap, highlightSelectionMatches } from "@codemirror/search";
import {
  closeBrackets,
  closeBracketsKeymap,
} from "@codemirror/autocomplete";
import {
  lintGutter,
  setDiagnostics,
  type Diagnostic as CodeMirrorDiagnostic,
} from "@codemirror/lint";
import type { Text } from "@codemirror/state";
import {
  csAutocompletion,
  csEditorTheme,
  csHighlighting,
  csLanguage,
} from "./cs-language";
import type { ProfileDiagnostic } from "./types";

export interface ProfileEditorHandle {
  focus: () => void;
  goToLine: (line: number, column?: number) => void;
}

interface ProfileEditorProps {
  value: string;
  onChange: (value: string) => void;
  diagnostics?: ProfileDiagnostic[];
  onSave?: () => void;
  ariaLabel?: string;
  placeholderText?: string;
}

const EMPTY_DIAGNOSTICS: ProfileDiagnostic[] = [];

function toCodeMirrorDiagnostic(doc: Text, diagnostic: ProfileDiagnostic): CodeMirrorDiagnostic {
  const lineNumber = Math.min(Math.max(1, diagnostic.line), doc.lines);
  const line = doc.line(lineNumber);
  const column = Math.max(1, diagnostic.column || 1);
  const from = Math.min(line.to, line.from + Math.min(column - 1, line.length));
  const to = Math.min(line.to, from + 1);
  return {
    from,
    to,
    severity: diagnostic.severity === "error" ? "error" : "warning",
    message: diagnostic.message,
    source: diagnostic.code,
  };
}

export const ProfileEditor = forwardRef<ProfileEditorHandle, ProfileEditorProps>(
  function ProfileEditor(
    { value, onChange, diagnostics = EMPTY_DIAGNOSTICS, onSave, ariaLabel, placeholderText },
    ref,
  ) {
    const hostRef = useRef<HTMLDivElement | null>(null);
    const viewRef = useRef<EditorView | null>(null);
    const onChangeRef = useRef(onChange);
    onChangeRef.current = onChange;
    const onSaveRef = useRef(onSave);
    onSaveRef.current = onSave;

    useImperativeHandle(ref, () => ({
      focus: () => viewRef.current?.focus(),
      goToLine: (line, column = 1) => {
        const view = viewRef.current;
        if (!view) return;
        const lineNumber = Math.min(Math.max(1, line), view.state.doc.lines);
        const info = view.state.doc.line(lineNumber);
        const pos = Math.min(info.to, info.from + Math.max(0, column - 1));
        view.dispatch({ selection: { anchor: pos }, scrollIntoView: true });
        view.focus();
      },
    }), []);

    useEffect(() => {
      const host = hostRef.current;
      if (!host) return;
      const state = EditorState.create({
        doc: value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          history(),
          foldGutter(),
          drawSelection(),
          dropCursor(),
          rectangularSelection(),
          crosshairCursor(),
          indentOnInput(),
          bracketMatching(),
          closeBrackets(),
          highlightSelectionMatches(),
          lintGutter(),
          csAutocompletion,
          csLanguage,
          csHighlighting,
          csEditorTheme,
          EditorView.lineWrapping,
          EditorView.contentAttributes.of({
            "aria-label": ariaLabel ?? "Malleable C2 profile editor",
            spellcheck: "false",
            autocapitalize: "off",
            autocomplete: "off",
          }),
          placeholder(placeholderText ?? "set sleeptime \"60000\";"),
          keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...historyKeymap,
            ...foldKeymap,
            ...searchKeymap,
            indentWithTab,
            {
              key: "Mod-s",
              run: () => {
                onSaveRef.current?.();
                return true;
              },
            },
          ]),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
        ],
      });
      const view = new EditorView({ state, parent: host });
      viewRef.current = view;
      return () => {
        view.destroy();
        viewRef.current = null;
      };
      // value is intentionally read once; later edits are synced below.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [ariaLabel, placeholderText]);

    useEffect(() => {
      const view = viewRef.current;
      if (!view) return;
      const current = view.state.doc.toString();
      if (current !== value) {
        view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
      }
    }, [value]);

    useEffect(() => {
      const view = viewRef.current;
      if (!view) return;
      const mapped = diagnostics.map((diagnostic) => toCodeMirrorDiagnostic(view.state.doc, diagnostic));
      view.dispatch(setDiagnostics(view.state, mapped));
    }, [diagnostics]);

    return <div ref={hostRef} className="h-[520px] min-h-[360px] overflow-hidden rounded-lg border border-border bg-background/60" />;
  },
);
