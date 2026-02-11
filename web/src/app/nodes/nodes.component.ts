import { Component, OnDestroy, OnInit } from '@angular/core';
import { Observable, Subscription } from 'rxjs';
import { shareReplay, switchMap, map } from 'rxjs/operators';
import { NodeView, VisualizerService } from '../visualizer.service';
import { RefreshService } from '../refresh.service';

@Component({
  selector: 'app-nodes',
  templateUrl: './nodes.component.html',
  styleUrls: ['./nodes.component.scss']
})
export class NodesComponent implements OnInit, OnDestroy {

  nodes$: Observable<NodeView[]> | null = null;
  filteredNodes$: Observable<NodeView[]> | null = null;

  // Filter state
  filterText = '';
  showGpuOnly = false;

  constructor(
    private service: VisualizerService,
    private refreshService: RefreshService,
  ) { }

  ngOnInit(): void {
    this.nodes$ = this.refreshService.tick$.pipe(
      switchMap(() => this.service.getNodes()),
      shareReplay(1),
    );

    this.applyFilters();
  }

  ngOnDestroy(): void {
    // Async pipe handles unsubscription
  }

  applyFilters(): void {
    if (!this.nodes$) return;

    this.filteredNodes$ = this.nodes$.pipe(
      map(nodes => {
        if (!nodes) return [];
        return nodes.filter(n => {
          const matchesText = n.name.toLowerCase().includes(this.filterText.toLowerCase());
          const matchesGpu = this.showGpuOnly ? (n.gpuSlots && n.gpuSlots.length > 0) : true;
          return matchesText && matchesGpu;
        });
      })
    );
  }

  onFilterChange(text: string): void {
    this.filterText = text;
    this.applyFilters();
  }

  toggleGpuFilter(): void {
    this.showGpuOnly = !this.showGpuOnly;
    this.applyFilters();
  }
}
