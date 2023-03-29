import { useState, useEffect } from "preact/compat";

const useDropzone = (node: HTMLElement | null): {dragging: boolean, files: File[]} => {
  const [files, setFiles] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);

  const handleAddFiles = (fileList: FileList) => {
    const filesArray = Array.from(fileList || []);
    setFiles(filesArray);
  };

  // handle drag events
  const handleDrag = (e: DragEvent) => {
    e.preventDefault();
    e.stopPropagation();

    if (e.type === "dragenter" || e.type === "dragover") {
      setDragging(true);
    } else if (e.type === "dragleave") {
      setDragging(false);
    }
  };

  // triggers when file is dropped
  const handleDrop = (e: DragEvent) => {
    e.preventDefault();
    e.stopPropagation();

    setDragging(false);
    if (e?.dataTransfer?.files && e.dataTransfer.files[0]) {
      handleAddFiles(e.dataTransfer.files);
    }
  };

  // triggers when file is pasted
  const handlePaste = (e: ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    const jsonFiles = Array.from(items)
      .filter(item => item.type === "application/json")
      .map(item => item.getAsFile())
      .filter(file => file !== null) as File[];
    setFiles(jsonFiles);
  };

  useEffect(() => {
    node?.addEventListener("dragenter", handleDrag);
    node?.addEventListener("dragleave", handleDrag);
    node?.addEventListener("dragover", handleDrag);
    node?.addEventListener("drop", handleDrop);
    node?.addEventListener("paste", handlePaste);

    return () => {
      node?.removeEventListener("dragenter", handleDrag);
      node?.removeEventListener("dragleave", handleDrag);
      node?.removeEventListener("dragover", handleDrag);
      node?.removeEventListener("drop", handleDrop);
      node?.removeEventListener("paste", handlePaste);
    };
  }, [node]);

  return {
    files,
    dragging,
  };
};

export default useDropzone;
