import { createContext, useContext } from 'react';

// SessionContext holds GET /api/session: the sign-in mode and who is signed in
// ({ email, name, picture } with Google, empty with the shared password).
export const SessionContext = createContext(null);

export function useSession() {
    return useContext(SessionContext);
}
