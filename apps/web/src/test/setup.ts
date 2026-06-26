import "@testing-library/jest-dom/vitest";

// jsdom 24 does not implement Blob.prototype.text() — polyfill via FileReader
// (FileReader IS implemented in jsdom). Without this, File.text() throws in tests.
if (typeof Blob !== "undefined" && !Blob.prototype.text) {
  Blob.prototype.text = function (): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.addEventListener("load", () => resolve(reader.result as string));
      reader.addEventListener("error", () => reject(reader.error));
      reader.readAsText(this);
    });
  };
}
