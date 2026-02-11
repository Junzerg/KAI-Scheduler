import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';

import { BrowserAnimationsModule } from '@angular/platform-browser/animations';
import { HttpClientModule, HTTP_INTERCEPTORS } from '@angular/common/http';

import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import { JobsComponent } from './jobs/jobs.component';
import { DashboardComponent } from './dashboard/dashboard.component';
import { SharedModule } from './shared.module';
import { NodesComponent } from './nodes/nodes.component';
import { NodeCardComponent } from './nodes/node-card/node-card.component';
import { GpuSlotsComponent } from './nodes/gpu-slots/gpu-slots.component';
import { NodeGridComponent } from './nodes/node-grid/node-grid.component';
import { QueuesComponent } from './queues/queues.component';
import { QueueResourceBarComponent } from './queues/queue-resource-bar/queue-resource-bar.component';
import { ErrorBannerComponent } from './error-banner/error-banner.component';
import { JobDonutChartComponent } from './dashboard/job-donut-chart/job-donut-chart.component';
import { ApiErrorInterceptor } from './error.interceptor';

@NgModule({
  declarations: [
    AppComponent,
    DashboardComponent,
    JobsComponent,
    NodesComponent,
    NodeCardComponent,
    GpuSlotsComponent,
    NodeGridComponent,
    QueuesComponent,
    QueueResourceBarComponent,
    ErrorBannerComponent,
    JobDonutChartComponent
  ],
  imports: [
    BrowserModule,
    BrowserAnimationsModule,
    HttpClientModule,
    AppRoutingModule,
    SharedModule
  ],
  providers: [
    { provide: HTTP_INTERCEPTORS, useClass: ApiErrorInterceptor, multi: true }
  ],
  bootstrap: [AppComponent]
})
export class AppModule { }
