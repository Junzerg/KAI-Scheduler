import { Component, OnInit, ViewChild, AfterViewInit } from '@angular/core';
import { MatTableDataSource } from '@angular/material/table';
import { MatPaginator } from '@angular/material/paginator';
import { MatSort } from '@angular/material/sort';
import { VisualizerService, JobView } from '../visualizer.service';
import { NamespaceService } from '../namespace.service';
import { switchMap, catchError } from 'rxjs/operators';
import { of } from 'rxjs';
import { animate, state, style, transition, trigger } from '@angular/animations';

@Component({
    selector: 'app-jobs',
    templateUrl: './jobs.component.html',
    styleUrls: ['./jobs.component.scss'],
    animations: [
        trigger('detailExpand', [
            state('collapsed', style({ height: '0px', minHeight: '0' })),
            state('expanded', style({ height: '*' })),
            transition('expanded <=> collapsed', animate('225ms cubic-bezier(0.4, 0.0, 0.2, 1)')),
        ]),
    ],
})
export class JobsComponent implements OnInit, AfterViewInit {

    displayedColumns: string[] = ['name', 'namespace', 'status', 'queue', 'createTime', 'tasks'];
    dataSource: MatTableDataSource<JobView>;
    expandedElement: JobView | null | undefined;
    isLoadingResults = true;

    @ViewChild(MatPaginator) paginator!: MatPaginator;
    @ViewChild(MatSort) sort!: MatSort;

    constructor(
        private visualizerService: VisualizerService,
        private namespaceService: NamespaceService
    ) {
        this.dataSource = new MatTableDataSource();
    }

    ngOnInit(): void {
        // Subscribe to namespace changes and refresh data
        this.namespaceService.selectedNamespace$.pipe(
            switchMap(ns => {
                this.isLoadingResults = true;
                return this.visualizerService.getJobs(ns).pipe(
                    catchError(() => {
                        this.isLoadingResults = false;
                        return of([]);
                    })
                );
            })
        ).subscribe(data => {
            this.isLoadingResults = false;
            this.dataSource.data = data;
            // Re-apply filter if needed or just reset page
            if (this.paginator) {
                this.paginator.firstPage();
            }
        });
    }

    ngAfterViewInit() {
        this.dataSource.paginator = this.paginator;
        this.dataSource.sort = this.sort;

        // Custom filter predicate if we want to filter by multiple fields
        this.dataSource.filterPredicate = (data: JobView, filter: string) => {
            const dataStr = (data.name + data.namespace + data.status).toLowerCase();
            return dataStr.indexOf(filter.toLowerCase()) != -1;
        };
    }

    applyFilter(event: Event) {
        const filterValue = (event.target as HTMLInputElement).value;
        this.dataSource.filter = filterValue.trim().toLowerCase();

        if (this.dataSource.paginator) {
            this.dataSource.paginator.firstPage();
        }
    }

    getStatusColor(status: string): string {
        switch (status.toLowerCase()) {
            case 'running': return 'primary'; // Will use primary color
            case 'pending': return 'accent'; // Will use accent color
            case 'failed': return 'warn';
            case 'completed': return 'default';
            default: return 'default';
        }
    }
}
