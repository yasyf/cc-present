import { useEffect, useRef } from 'react';
import { KEYMAP } from '../keymap';

// HelpOverlay is a native modal dialog listing the keymap straight from KEYMAP,
// so the shortcuts and their documentation can never drift. showModal drives the
// open state imperatively; the dialog's own close event (native Esc, backdrop)
// syncs back so the provider's helpOpen stays honest.
export function HelpOverlay({ open, onClose }: { open: boolean; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const dialog = ref.current;
    if (!dialog) return;
    if (open && !dialog.open) dialog.showModal();
    else if (!open && dialog.open) dialog.close();
  }, [open]);

  return (
    <dialog ref={ref} className="help-dialog" onClose={onClose}>
      <div className="help-head">
        <h2 className="help-title">Keyboard shortcuts</h2>
        <button type="button" className="link-btn" onClick={onClose}>
          Close
        </button>
      </div>
      <table className="help-table">
        <tbody>
          {KEYMAP.map((row) => (
            <tr key={row.action}>
              <td className="help-keys">
                {row.keys.map((k) => (
                  <kbd key={k}>{k}</kbd>
                ))}
              </td>
              <td className="help-ctx">{row.context}</td>
              <td className="help-action">{row.action}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </dialog>
  );
}
