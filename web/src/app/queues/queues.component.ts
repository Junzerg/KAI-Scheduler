import { Component, OnInit } from '@angular/core';
import { FlatTreeControl } from '@angular/cdk/tree';
import { MatTreeFlatDataSource, MatTreeFlattener } from '@angular/material/tree';
import { VisualizerService, QueueView } from '../visualizer.service';
import { BehaviorSubject, timer } from 'rxjs';
import { switchMap } from 'rxjs/operators';

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
export class QueuesComponent implements OnInit {

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

    constructor(private visualizerService: VisualizerService) { }

    ngOnInit(): void {
        // Auto-refresh every 5 seconds
        timer(0, 5000).pipe(
            switchMap(() => this.visualizerService.getQueues())
        ).subscribe(data => {
            this.dataSource.data = data;
            // Optionally expand all by default? Or keep state?
            // Keeping state with MatTree is tricky on full refresh. 
            // For Phase 1, we might just collapse or expand all.
            if (this.treeControl.dataNodes && this.treeControl.dataNodes.length > 0) {
                this.treeControl.expandAll();
            }
        });
    }

    hasChild = (_: number, node: QueueFlatNode) => node.expandable;
}
