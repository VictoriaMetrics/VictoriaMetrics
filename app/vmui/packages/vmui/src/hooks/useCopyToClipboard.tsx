import { useSnack } from "../contexts/Snackbar";

type CopyFn = (text: string, msgInfo?: string) => Promise<boolean> // Return success

const useCopyToClipboard = (): CopyFn => {
  const { showInfoMessage } = useSnack();

  return async (text, msgInfo) => {
    if (!navigator?.clipboard) {
      showInfoMessage({ text: <DebugInfoClipboardApi/>, type: "error", timeout: 20000 });
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

const DebugInfoClipboardApi = () => (
  <div className="vm-snackbar-details">
    <p className="vm-snackbar-details__title">Clipboard not supported</p>
    {!window.isSecureContext ? (
      <p className="vm-snackbar-details__msg">
        <p>This page is not running in a secure context (HTTPS).</p>
        <p>Clipboard operations require a secure context.</p>
        <a
          className="vm-link vm-link_underlined vm-link_colored"
          href="https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts"
          target="_blank"
          rel="noopener noreferrer"
        >
          Learn more about secure contexts
        </a>
      </p>
    ) : (
      <p className="vm-snackbar-details__msg">
        <p>Common reasons:</p>
        <ul>
          <li>Browser restrictions</li>
          <li>Insecure connection (HTTP)</li>
          <li>Permissions not granted</li>
        </ul>
        <p>
          For detailed information, visit the <a
            className="vm-link vm-link_underlined vm-link_colored"
            href="https://developer.mozilla.org/en-US/docs/Web/API/Clipboard_API#security_considerations"
            target="_blank"
            rel="noopener noreferrer"
          >
          Clipboard API documentation
          </a>
        </p>
      </p>
    )}
  </div>
);
