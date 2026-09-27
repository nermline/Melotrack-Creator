import { createContext, useContext } from 'react';

export const FeedbackCtx = createContext(null);

// useFeedback returns { toast(message, tone), confirm({ title, message, ... }) }.
export function useFeedback() {
    return useContext(FeedbackCtx);
}
