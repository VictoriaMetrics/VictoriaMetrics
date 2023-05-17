import { useState } from "preact/compat";
import useEventListener from "./useEventListener";
import { useRef } from "react";

const useDropzone = (): { dragging: boolean, files: File[] } => {
  const [files, setFiles] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);
  const bodyRef = useRef(document.body);

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

  useEventListener("dragenter", handleDrag, bodyRef);
  useEventListener("dragleave", handleDrag, bodyRef);
  useEventListener("dragover", handleDrag, bodyRef);
  useEventListener("drop", handleDrop, bodyRef);
  useEventListener("paste", handlePaste, bodyRef);

  return {
    files,
    dragging,
  };
};

export default useDropzone;
