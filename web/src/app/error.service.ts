import { Injectable } from '@angular/core';
import { BehaviorSubject, Observable } from 'rxjs';

/**
 * Manages the global error state displayed by ErrorBannerComponent.
 */
@Injectable({ providedIn: 'root' })
export class ErrorService {

    private readonly _error$ = new BehaviorSubject<string | null>(null);
    private clearTimer: ReturnType<typeof setTimeout> | null = null;

    /** Current error message (null = no error). */
    readonly error$: Observable<string | null> = this._error$.asObservable();

    /**
     * Surface an error message.  Auto-clears after `autoClearMs` (default 15 s)
     * unless another error arrives first.
     */
    setError(message: string, autoClearMs = 15_000): void {
        this._error$.next(message);
        if (this.clearTimer) {
            clearTimeout(this.clearTimer);
        }
        this.clearTimer = setTimeout(() => this.clearError(), autoClearMs);
    }

    clearError(): void {
        this._error$.next(null);
        if (this.clearTimer) {
            clearTimeout(this.clearTimer);
            this.clearTimer = null;
        }
    }
}
