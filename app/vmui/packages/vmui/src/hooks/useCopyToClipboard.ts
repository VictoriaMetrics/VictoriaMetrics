import { useSnack } from "../contexts/Snackbar";

type CopyFn = (text: string, msgInfo?: string) => Promise<boolean> // Return success

const useCopyToClipboard = (): CopyFn => {
  const { showInfoMessage } = useSnack();

  return async (text, msgInfo) => {
    if (!navigator?.clipboard) {
      showInfoMessage({ text: "Clipboard not supported", type: "error" });
      console.warn("Clipboard not supported");
      return false;
    }

    // Try to save to clipboard then save it in the state if worked
    try {
      await navigator.clipboard.writeText(text);
      if (msgInfo) {
        showInfoMessage({ text: msgInfo, type: "success" });
      }
      return true;
    } catch (error) {
      if (error instanceof Error) {
        showInfoMessage({ text: `${error.name}: ${error.message}`, type: "error" });
      }
      console.warn("Copy failed", error);
      return false;
    }
  };
};

export default useCopyToClipboard;
