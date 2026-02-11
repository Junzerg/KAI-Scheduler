import { Component, OnInit, OnDestroy } from '@angular/core';
import { FlatTreeControl } from '@angular/cdk/tree';
import { MatTreeFlatDataSource, MatTreeFlattener } from '@angular/material/tree';
import { VisualizerService, QueueView } from '../visualizer.service';
import { RefreshService } from '../refresh.service';
import { Subscription } from 'rxjs';
import { switchMap, catchError } from 'rxjs/operators';
import { of } from 'rxjs';

/** Flat node with expandable and level information */
interface QueueFlatNode {
    expandable: boolean;
    name: string;
    level: number;
    weight: number;
    // Raw data for resource bars
    resources: any; // QueueResources
}

@Component({
    selector: 'app-queues',
    templateUrl: './queues.component.html',
    styleUrls: ['./queues.component.scss']
})
export class QueuesComponent implements OnInit, OnDestroy {

    // Transformer: Map nested QueueView to flat QueueFlatNode
    private _transformer = (node: QueueView, level: number) => {
        return {
            expandable: !!node.children && node.children.length > 0,
            name: node.name,
            level: level,
            weight: node.weight,
            resources: node.resources
        };
    };

    treeControl = new FlatTreeControl<QueueFlatNode>(
        node => node.level,
        node => node.expandable,
    );

    treeFlattener = new MatTreeFlattener(
        this._transformer,
        node => node.level,
        node => node.expandable,
        node => node.children,
    );

    dataSource = new MatTreeFlatDataSource(this.treeControl, this.treeFlattener);

    private sub!: Subscription;

    constructor(
        private visualizerService: VisualizerService,
        private refreshService: RefreshService,
    ) { }

    ngOnInit(): void {
        this.sub = this.refreshService.tick$.pipe(
            switchMap(() => this.visualizerService.getQueues().pipe(
                catchError(() => of([]))
            ))
        ).subscribe(data => {
            this.dataSource.data = data;
            if (this.treeControl.dataNodes && this.treeControl.dataNodes.length > 0) {
                this.treeControl.expandAll();
            }
        });
    }

    ngOnDestroy(): void {
        this.sub?.unsubscribe();
    }

    hasChild = (_: number, node: QueueFlatNode) => node.expandable;
}
