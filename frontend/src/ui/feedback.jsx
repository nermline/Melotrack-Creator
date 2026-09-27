import { useCallback, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { CircleCheck, TriangleAlert, Info } from 'lucide-react';
import { Button, Modal } from './index';

import { FeedbackCtx } from './feedbackContext';

// FeedbackProvider offers toast() and confirm() without window.alert/confirm.
export function FeedbackProvider({ children }) {
    const [toasts, setToasts] = useState([]);
    const [dialog, setDialog] = useState(null);
    const idRef = useRef(0);

    const toast = useCallback((message, tone = 'ok') => {
        const id = ++idRef.current;
        setToasts((t) => [...t.slice(-3), { id, message, tone }]);
        setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), tone === 'bad' ? 6000 : 3200);
    }, []);

    const confirm = useCallback(
        (opts) =>
            new Promise((resolve) => {
                setDialog({ ...opts, resolve });
            }),
        [],
    );

    const close = (result) => {
        dialog?.resolve(result);
        setDialog(null);
    };

    const icons = { ok: CircleCheck, bad: TriangleAlert, info: Info };
    const colors = { ok: 'text-ok', bad: 'text-bad', info: 'text-info' };

    return (
        <FeedbackCtx.Provider value={{ toast, confirm }}>
            {children}
            <div className="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4">
                <AnimatePresence>
                    {toasts.map((t) => {
                        const Icon = icons[t.tone] || Info;
                        return (
                            <motion.div
                                key={t.id}
                                layout
                                initial={{ opacity: 0, y: 16, scale: 0.96 }}
                                animate={{ opacity: 1, y: 0, scale: 1 }}
                                exit={{ opacity: 0, y: 8 }}
                                className="panel pointer-events-auto flex max-w-md items-start gap-2.5 px-4 py-3 text-sm shadow-2xl"
                            >
                                <Icon size={18} className={`mt-px shrink-0 ${colors[t.tone] || ''}`} />
                                <span className="whitespace-pre-line">{t.message}</span>
                            </motion.div>
                        );
                    })}
                </AnimatePresence>
            </div>
            <Modal
                open={!!dialog}
                onClose={() => close(false)}
                title={dialog?.title || 'Підтвердіть дію'}
                width={440}
                footer={
                    <>
                        <Button variant="ghost" onClick={() => close(false)}>
                            Скасувати
                        </Button>
                        <Button variant={dialog?.danger ? 'danger' : 'primary'} onClick={() => close(true)} autoFocus>
                            {dialog?.confirmLabel || 'Так'}
                        </Button>
                    </>
                }
            >
                <p className="m-0 text-sm leading-relaxed text-dim">{dialog?.message}</p>
            </Modal>
        </FeedbackCtx.Provider>
    );
}
