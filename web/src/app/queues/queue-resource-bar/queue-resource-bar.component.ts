import { Component, Input, OnInit, OnChanges, SimpleChanges } from '@angular/core';

@Component({
    selector: 'app-queue-resource-bar',
    templateUrl: './queue-resource-bar.component.html',
    styleUrls: ['./queue-resource-bar.component.scss']
})
export class QueueResourceBarComponent implements OnInit, OnChanges {
    @Input() resourceName: string = '';
    @Input() unit: string = '';

    // Raw values
    @Input() guaranteed: number = 0;
    @Input() allocated: number = 0;
    @Input() max: number = 0; // If 0, treated as no limit (or hidden)

    // Display properties
    usagePercent: number = 0;
    guaranteedPercent: number = 0;

    // Formatted display strings
    displayAllocated: string = '0';
    displayGuaranteed: string = '0';
    displayMax: string = '';

    // Color state
    statusColor: 'primary' | 'accent' | 'warn' = 'primary';

    // Tooltip
    tooltipText: string = '';

    constructor() { }

    ngOnInit(): void {
        this.calculateStats();
    }

    ngOnChanges(changes: SimpleChanges): void {
        this.calculateStats();
    }

    private calculateStats(): void {
        // 1. Determine the scale denominator (100% width)
        let scaleBase = this.max;
        if (this.max <= 0) {
            scaleBase = Math.max(this.guaranteed, this.allocated, 1) * 1.2;
        }
        if (scaleBase <= 0) scaleBase = 1;

        // 2. Calculate percentages
        this.usagePercent = Math.min((this.allocated / scaleBase) * 100, 100);
        this.guaranteedPercent = Math.min((this.guaranteed / scaleBase) * 100, 100);

        // 3. Determine Color
        if (this.allocated > this.max && this.max > 0) {
            this.statusColor = 'warn';
        } else if (this.allocated > this.guaranteed && this.guaranteed > 0) {
            this.statusColor = 'accent';
        } else {
            this.statusColor = 'primary';
        }

        // 4. Format display values
        this.displayAllocated = this.formatValue(this.allocated);
        this.displayGuaranteed = this.formatValue(this.guaranteed);
        this.displayMax = this.max > 0 ? this.formatValue(this.max) : '';

        // 5. Tooltip
        this.tooltipText = `${this.resourceName}:  Used: ${this.displayAllocated}  Guaranteed: ${this.displayGuaranteed}  Max: ${this.max > 0 ? this.displayMax : 'Unlimited'}`;
    }

    formatValue(val: number): string {
        if (this.unit === 'm') {
            // CPU milli-cores
            if (val >= 1000) {
                return `${(val / 1000).toFixed(1)} cores`;
            }
            return `${val}m`;
        } else if (this.unit === 'GiB') {
            // Memory: raw bytes → human-readable
            if (val === 0) return '0';
            const gib = val / (1024 * 1024 * 1024);
            if (gib >= 1) {
                return `${gib.toFixed(2)} GiB`;
            }
            const mib = val / (1024 * 1024);
            if (mib >= 1) {
                return `${mib.toFixed(1)} MiB`;
            }
            const kib = val / 1024;
            return `${kib.toFixed(0)} KiB`;
        } else if (this.unit === 'gpu') {
            return `${val}`;
        } else {
            return val.toString();
        }
    }
}
