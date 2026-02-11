import { Component } from '@angular/core';
import { ErrorService } from '../error.service';
import { animate, style, transition, trigger } from '@angular/animations';

@Component({
    selector: 'app-error-banner',
    templateUrl: './error-banner.component.html',
    styleUrls: ['./error-banner.component.scss'],
    animations: [
        trigger('slideIn', [
            transition(':enter', [
                style({ transform: 'translateY(-100%)', opacity: 0 }),
                animate('250ms ease-out', style({ transform: 'translateY(0)', opacity: 1 })),
            ]),
            transition(':leave', [
                animate('200ms ease-in', style({ transform: 'translateY(-100%)', opacity: 0 })),
            ]),
        ]),
    ],
})
export class ErrorBannerComponent {
    constructor(public errorService: ErrorService) { }
}
