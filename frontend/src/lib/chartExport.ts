import { toPng } from "html-to-image";

export async function exportElementPng(element: HTMLElement, filename: string): Promise<void> {
  const dataUrl = await toPng(element, {
    cacheBust: true,
    pixelRatio: 2,
    backgroundColor: getComputedStyle(element).backgroundColor || "#ffffff",
  });
  const link = document.createElement("a");
  link.download = filename;
  link.href = dataUrl;
  link.click();
}