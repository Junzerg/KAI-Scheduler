import { Injectable } from '@angular/core';
import {
    HttpInterceptor,
    HttpRequest,
    HttpHandler,
    HttpEvent,
} from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { ErrorService } from './error.service';

/**
 * HTTP interceptor that catches API errors and broadcasts them via ErrorService.
 */
@Injectable()
export class ApiErrorInterceptor implements HttpInterceptor {

    constructor(private errorService: ErrorService) { }

    intercept(
        req: HttpRequest<unknown>,
        next: HttpHandler,
    ): Observable<HttpEvent<unknown>> {
        return next.handle(req).pipe(
            catchError(err => {
                let msg: string;
                if (err.status === 0) {
                    msg = 'API server is unreachable. Check if KAI Scheduler is running.';
                } else if (err.status >= 500) {
                    msg = `Server error: ${err.status} ${err.statusText}`;
                } else if (err.status >= 400) {
                    msg = `Request error: ${err.status} ${err.statusText}`;
                } else {
                    msg = `Unexpected error: ${err.message || 'Unknown'}`;
                }
                this.errorService.setError(msg);
                return throwError(() => err);
            }),
        );
    }
}
