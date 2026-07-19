export async function exportElementPng(element: HTMLElement, filename: string): Promise<void> {
  // Dynamically imported so html-to-image is only fetched when an export is triggered.
  const { toPng } = await import("html-to-image");
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