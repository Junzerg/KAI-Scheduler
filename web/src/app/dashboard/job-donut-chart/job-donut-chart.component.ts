import { Component, Input, OnChanges } from '@angular/core';
import { Router } from '@angular/router';

interface ArcSegment {
    status: string;
    count: number;
    percent: number;
    color: string;
    startAngle: number;   // degrees
    endAngle: number;
    path: string;          // SVG arc path
    labelX: number;
    labelY: number;
}

const STATUS_COLORS: { [key: string]: string } = {
    Running: '#4caf50',
    Pending: '#ff9800',
    Failed: '#f44336',
    Completed: '#2196f3',
    Unknown: '#9e9e9e',
};

@Component({
    selector: 'app-job-donut-chart',
    templateUrl: './job-donut-chart.component.html',
    styleUrls: ['./job-donut-chart.component.scss'],
})
export class JobDonutChartComponent implements OnChanges {

    @Input() jobCounts: { [status: string]: number } = {};

    segments: ArcSegment[] = [];
    total = 0;
    hoveredSegment: ArcSegment | null = null;

    // SVG constants
    readonly cx = 100;
    readonly cy = 100;
    readonly outerR = 90;
    readonly innerR = 55;

    constructor(private router: Router) { }

    ngOnChanges(): void {
        this.buildSegments();
    }

    onSegmentClick(seg: ArcSegment): void {
        this.router.navigate(['/jobs'], {
            queryParams: { status: seg.status },
        });
    }

    onSegmentHover(seg: ArcSegment | null): void {
        this.hoveredSegment = seg;
    }

    // ──────────────── SVG arc helpers ────────────────

    private buildSegments(): void {
        const entries = Object.entries(this.jobCounts || {}).filter(([, v]) => v > 0);
        this.total = entries.reduce((sum, [, v]) => sum + v, 0);

        if (this.total === 0) {
            this.segments = [];
            return;
        }

        let currentAngle = -90; // start at 12 o'clock
        this.segments = entries.map(([status, count]) => {
            const percent = count / this.total;
            const sweep = percent * 360;
            const startAngle = currentAngle;
            const endAngle = currentAngle + sweep;

            const path = this.describeArc(startAngle, endAngle);

            // label position: midpoint of the arc, halfway between inner and outer
            const midAngle = ((startAngle + endAngle) / 2) * (Math.PI / 180);
            const labelR = (this.outerR + this.innerR) / 2;
            const labelX = this.cx + labelR * Math.cos(midAngle);
            const labelY = this.cy + labelR * Math.sin(midAngle);

            currentAngle = endAngle;

            return {
                status,
                count,
                percent,
                color: STATUS_COLORS[status] || STATUS_COLORS['Unknown'],
                startAngle,
                endAngle,
                path,
                labelX,
                labelY,
            };
        });
    }

    /**
     * Build an SVG `<path d="...">` string for a donut arc from `startDeg` to
     * `endDeg` (clockwise, 0 = 3 o'clock).
     */
    private describeArc(startDeg: number, endDeg: number): string {
        // Clamp to avoid full-circle edge case with identical start/end
        let sweep = endDeg - startDeg;
        if (sweep >= 360) { sweep = 359.999; }

        const startRad = (startDeg * Math.PI) / 180;
        const endRad = ((startDeg + sweep) * Math.PI) / 180;
        const largeArc = sweep > 180 ? 1 : 0;

        const ox1 = this.cx + this.outerR * Math.cos(startRad);
        const oy1 = this.cy + this.outerR * Math.sin(startRad);
        const ox2 = this.cx + this.outerR * Math.cos(endRad);
        const oy2 = this.cy + this.outerR * Math.sin(endRad);
        const ix1 = this.cx + this.innerR * Math.cos(endRad);
        const iy1 = this.cy + this.innerR * Math.sin(endRad);
        const ix2 = this.cx + this.innerR * Math.cos(startRad);
        const iy2 = this.cy + this.innerR * Math.sin(startRad);

        return [
            `M ${ox1} ${oy1}`,
            `A ${this.outerR} ${this.outerR} 0 ${largeArc} 1 ${ox2} ${oy2}`,
            `L ${ix1} ${iy1}`,
            `A ${this.innerR} ${this.innerR} 0 ${largeArc} 0 ${ix2} ${iy2}`,
            'Z',
        ].join(' ');
    }
}
