export async function exportElementPng(element: HTMLElement, filename: string): Promise<void> {
  // Dynamically imported so html-to-image is only fetched when an export is triggered.
  const { toPng } = await import("html-to-image");
  const bg = getComputedStyle(element).backgroundColor;
  const bgSafe = !bg || bg === "rgba(0, 0, 0, 0)" || bg === "transparent" ? "#ffffff" : bg;
  const dataUrl = await toPng(element, {
    cacheBust: true,
    pixelRatio: 2,
    backgroundColor: bgSafe,
  });
  const link = document.createElement("a");
  link.download = filename;
  link.href = dataUrl;
  link.click();
}