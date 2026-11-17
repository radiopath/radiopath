import { toast } from 'svelte-sonner';
import { ApiError } from '#lib/api.ts';

/** Shows the error of an action as a toast: one invalid field as the message, several as a list below it. */
export function notifyError(err: unknown) {
	if (err instanceof ApiError && err.errors.length === 1) toast.error(err.errors[0]);
	else if (err instanceof ApiError && err.errors.length > 1) toast.error(err.message, { description: err.errors.join('\n') });
	else toast.error(err instanceof Error ? err.message : String(err));
}

/** An error for input the form rejects before asking the server, shown like the server's own. */
export const invalid = (...messages: string[]) => new ApiError(422, 'Invalid input', messages);
