import { Injectable } from '@angular/core';
import { BehaviorSubject, combineLatest, NEVER, Observable, timer } from 'rxjs';
import { switchMap } from 'rxjs/operators';

/**
 * Global auto-refresh service.
 *
 * All page components subscribe to `tick$` instead of creating their own
 * `timer()`.  The toolbar exposes a pause/resume button that controls
 * `paused$`, which immediately stops / resumes the tick for every listener.
 */
@Injectable({ providedIn: 'root' })
export class RefreshService {

    private readonly _paused$ = new BehaviorSubject<boolean>(false);
    private readonly _intervalMs$ = new BehaviorSubject<number>(5000);

    /** Emits an incrementing counter every `intervalMs` while NOT paused. */
    readonly tick$: Observable<number> = combineLatest([
        this._paused$,
        this._intervalMs$,
    ]).pipe(
        switchMap(([paused, ms]) => (paused ? NEVER : timer(0, ms))),
    );

    /** Whether auto-refresh is currently paused. */
    readonly isPaused$: Observable<boolean> = this._paused$.asObservable();

    get isPaused(): boolean {
        return this._paused$.value;
    }

    togglePause(): void {
        this._paused$.next(!this._paused$.value);
    }

    setPaused(paused: boolean): void {
        this._paused$.next(paused);
    }

    setInterval(ms: number): void {
        this._intervalMs$.next(ms);
    }
}
