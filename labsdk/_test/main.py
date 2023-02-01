# -*- coding: utf-8 -*-
#  Copyright (c) 2022 RaptorML authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.
from datetime import datetime

import pandas as pd
from typing_extensions import TypedDict

from labsdk.raptor import data_source, Context, feature, aggregation, AggregationFunction, freshness, model, manifests, \
    keep_previous, TrainingContext, StreamingConfig


# getting started code

@data_source(
    training_data=pd.read_parquet(
        'https://gist.github.com/AlmogBaku/8be77c2236836177b8e54fa8217411f2/raw/emails.parquet'),
    keys=['id', 'account_id'],
    timestamp='event_at',
    production_config=StreamingConfig(kind='kafka'),
)
class Email(TypedDict('Email', {'from': str})):
    event_at: datetime
    account_id: str
    subject: str
    to: str


@feature(keys='account_id', data_source=Email)
@aggregation(function=AggregationFunction.Count, over='10h', granularity='1h')
def emails_10h(this_row: Email, ctx: Context) -> int:
    """email over 10 hours"""
    return 1


print('# Emails')
print(f'```\n{Email.manifest()}\n```')
print('## Feature: `emails_10h`')
print(f'```\n{emails_10h.manifest()}\n```')
print('### Replayed')
print(emails_10h.replay().to_markdown())


@data_source(
    training_data=pd.read_csv(
        'https://gist.githubusercontent.com/AlmogBaku/8be77c2236836177b8e54fa8217411f2/raw/deals.csv'),
    keys=['id', 'account_id'],
    timestamp='event_at',
)
class Deal(TypedDict):
    id: int
    event_at: pd.Timestamp
    account_id: str
    amount: float


@feature(keys='account_id', data_source=Deal)
@aggregation(
    function=[AggregationFunction.Sum, AggregationFunction.Avg, AggregationFunction.Max, AggregationFunction.Min],
    over='10h',
    granularity='1m'
)
def deals_10h(this_row: Deal, ctx: Context) -> float:
    """sum/avg/min/max of deal amount over 10 hours"""
    return this_row['amount']


@feature(keys='account_id', sourceless_markers_df=Deal.raptor_spec.local_df)
@freshness(target='-1', invalid_after='-1')
def emails_deals(_, ctx: Context) -> float:
    """emails/deal[avg] rate over 10 hours"""
    e, _ = ctx.get_feature('emails_10h+count')
    d, _ = ctx.get_feature('deals_10h+avg')
    if e is None or d is None:
        return None
    return e / d


@feature(keys='account_id', data_source=Deal)
@freshness(target='1h', invalid_after='2h')
@keep_previous(versions=1, over='1h')
def last_amount(this_row: Deal, ctx: Context) -> float:
    return this_row['amount']


@feature(keys='account_id', sourceless_markers_df=Deal.raptor_spec.local_df)
@freshness(target='1h', invalid_after='2h')
def diff_with_previous_price(this_row: Deal, ctx: Context) -> float:
    lv, ts = ctx.get_feature('last_amount@-1')
    if lv is None:
        return 0
    return this_row['amount'] - lv


print('# Deals')
print(f'```\n{Deal.manifest()}\n```')
print('## Feature: `deals_10h`')
print(f'```\n{deals_10h.manifest()}\n```')
print(f'### Replayed')
print(deals_10h.replay().to_markdown())
print(f'## Feature: `emails_deals`')
print(f'```\n{emails_deals.manifest()}\n```')
print('### Replayed')
print(emails_deals.replay().to_markdown())
print(f'## Feature: `last_amount`')
print(f'```\n{last_amount.manifest()}\n```')
print('### Replayed')
print(last_amount.replay().to_markdown())
print(f'## Feature: `diff_with_previous_price`')
print(f'```\n{diff_with_previous_price.manifest()}\n```')
print('### Replayed')
print(diff_with_previous_price.replay().to_markdown())


@model(
    keys=['account_id'],
    input_features=[
        'emails_10h+count', 'deals_10h+sum', emails_deals, diff_with_previous_price
    ],
    input_labels=[last_amount],
    model_framework='sklearn',
    model_server='sagemaker-ack',
)
@freshness(target='1h', invalid_after='100h')
def deal_prediction(ctx: TrainingContext) -> float:
    from xgboost import XGBClassifier
    from sklearn.model_selection import train_test_split

    df = ctx.features_and_labels()
    X = df[ctx.input_features]
    y = df[ctx.input_labels]

    # Split the data into training and testing sets
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=0)

    # Initialize an XGBoost model
    xgb_model = XGBClassifier()

    # Fit the model to the training data
    xgb_model.fit(X_train, y_train)

    # Evaluate the model on the testing data
    accuracy = xgb_model.score(X_test, y_test)

    # Make sure the model has a minimum accuracy of 0.7
    if accuracy < 0.7:
        raise Exception('Accuracy is below 0.7')

    return xgb_model


print('# Model')
m = deal_prediction.train()
df = deal_prediction.features_and_labels(since=pd.to_datetime('2020-1-1'), until=pd.to_datetime('2022-12-31'))
print(df.to_markdown())


# counters
@feature(keys='account_id', data_source=Deal)
@aggregation(function=AggregationFunction.Count, over='9999984h', granularity='9999984h')
def views(this_row: Deal, ctx: Context) -> int:
    return 1


print('# Views')
print(f'```\n{views.manifest()}\n```')
print('## Replayed')
print(views.replay().to_markdown())

# gong
crm_records_df = pd.DataFrame.from_records([
    {'event_at': '2022-01-01 12:00:00+00:00', 'salesman_id': 'ada', 'action': 'deal_assigned', 'opportunity_id': 15},
    {'event_at': '2022-02-01 13:10:00+00:00', 'salesman_id': 'ada', 'action': 'deal_removed', 'opportunity_id': 15},
    {'event_at': '2022-04-01 13:20:00+00:00', 'salesman_id': 'ada', 'action': 'deal_assigned', 'opportunity_id': 15},
    {'event_at': '2022-06-01 14:00:00+00:00', 'salesman_id': 'ada', 'action': 'deal_closed', 'opportunity_id': 25},
    {'event_at': '2022-06-01 14:10:00+00:00', 'salesman_id': 'ada', 'action': 'deal_assigned', 'opportunity_id': 17},
    {'event_at': '2022-07-01 14:20:00+00:00', 'salesman_id': 'ada', 'action': 'deal_removed', 'opportunity_id': 17},
    {'event_at': '2022-08-01 14:30:00+00:00', 'salesman_id': 'ada', 'action': 'deal_assigned', 'opportunity_id': 17},
    {'event_at': '2022-09-01 14:40:00+00:00', 'salesman_id': 'ada', 'action': 'deal_closed', 'opportunity_id': 17},
    {'event_at': '2022-11-01 15:30:00+00:00', 'salesman_id': 'ada', 'action': 'deal_removed', 'opportunity_id': 17},
    {'event_at': '2022-01-01 12:00:00+00:00', 'salesman_id': 'brian', 'action': 'deal_assigned', 'opportunity_id': 132},
    {'event_at': '2022-02-01 12:20:00+00:00', 'salesman_id': 'brian', 'action': 'deal_removed', 'opportunity_id': 132},
    {'event_at': '2022-02-01 13:40:00+00:00', 'salesman_id': 'brian', 'action': 'deal_assigned', 'opportunity_id': 132},
    {'event_at': '2022-04-01 15:00:00+00:00', 'salesman_id': 'brian', 'action': 'deal_closed', 'opportunity_id': 132},
    {'event_at': '2022-05-01 15:10:00+00:00', 'salesman_id': 'brian', 'action': 'deal_removed', 'opportunity_id': 132},
    {'event_at': '2022-06-01 15:20:00+00:00', 'salesman_id': 'brian', 'action': 'deal_assigned', 'opportunity_id': 544},
    {'event_at': '2022-07-01 15:30:00+00:00', 'salesman_id': 'brian', 'action': 'deal_removed', 'opportunity_id': 544},
    {'event_at': '2022-08-01 15:40:00+00:00', 'salesman_id': 'brian', 'action': 'deal_assigned', 'opportunity_id': 544},
    {'event_at': '2022-09-01 15:50:00+00:00', 'salesman_id': 'brian', 'action': 'deal_closed', 'opportunity_id': 544},
    {'event_at': '2022-10-01 16:00:00+00:00', 'salesman_id': 'brian', 'action': 'deal_assigned', 'opportunity_id': 233},
    {'event_at': '2022-11-01 16:10:00+00:00', 'salesman_id': 'brian', 'action': 'deal_closed', 'opportunity_id': 233},
    {'event_at': '2022-12-01 16:20:00+00:00', 'salesman_id': 'brian', 'action': 'deal_assigned', 'opportunity_id': 444},
    {'event_at': '2022-12-01 16:30:00+00:00', 'salesman_id': 'brian', 'action': 'deal_closed', 'opportunity_id': 444},
])


@data_source(training_data=crm_records_df, keys=['salesman_id', 'opportunity_id'])
class CrmRecord(TypedDict):
    event_at: datetime
    salesman_id: str
    action: str
    opportunity_id: int


@feature(keys='salesman_id', data_source=CrmRecord)
@aggregation(function=AggregationFunction.DistinctCount, over='8760h', granularity='24h')
def unique_deals_involvement_annually(this_row: CrmRecord, ctx: Context) -> int:
    if this_row['action'] == 'deal_assigned':
        return this_row['opportunity_id']
    return None


unique_deals_involvement_annually.replay()


@feature(keys='salesman_id', data_source=CrmRecord)
@aggregation(function=AggregationFunction.DistinctCount, over='8760h', granularity='24h')
def closed_deals_annually(this_row: CrmRecord, ctx: Context) -> int:
    if this_row['action'] == 'deal_closed':
        return 1
    return None


closed_deals_annually.replay()


@feature(keys='salesman_id', data_source=CrmRecord)
@freshness(target='24h', invalid_after='8760h')
def salesperson_deals_closes_rate(this_row: CrmRecord, ctx: Context) -> int:
    udia, _ = ctx.get_feature('unique_deals_involvement_annually+distinct_count')
    cda, _ = ctx.get_feature('closed_deals_annually+count')
    if udia is None or cda is None:
        return None
    return udia / cda


salesperson_deals_closes_rate.replay()

# other tests


df = pd.DataFrame.from_records([
    {'event_at': '2022-01-01 12:00:00+00:00', 'account_id': 'ada', 'subject': 'wrote_code', 'commit_count': 1},
    {'event_at': '2022-01-01 13:10:00+00:00', 'account_id': 'ada', 'subject': 'wrote_code', 'commit_count': 1},
    {'event_at': '2022-01-01 13:20:00+00:00', 'account_id': 'ada', 'subject': 'fixed_bug', 'commit_count': 1},
    {'event_at': '2022-01-01 14:00:00+00:00', 'account_id': 'ada', 'subject': 'deployed', 'commit_count': 3},
    {'event_at': '2022-01-01 14:10:00+00:00', 'account_id': 'ada', 'subject': 'developed', 'commit_count': 1},
    {'event_at': '2022-01-01 14:20:00+00:00', 'account_id': 'ada', 'subject': 'built_model', 'commit_count': 4},
    {'event_at': '2022-01-01 14:30:00+00:00', 'account_id': 'ada', 'subject': 'wrote_code', 'commit_count': 3},
    {'event_at': '2022-01-01 14:40:00+00:00', 'account_id': 'ada', 'subject': 'experimented', 'commit_count': 2},
    {'event_at': '2022-01-01 15:30:00+00:00', 'account_id': 'ada', 'subject': 'wrote_code', 'commit_count': 1},
    {'event_at': '2022-01-01 12:00:00+00:00', 'account_id': 'brian', 'subject': 'developed', 'commit_count': 1},
    {'event_at': '2022-01-01 12:20:00+00:00', 'account_id': 'brian', 'subject': 'wrote_code', 'commit_count': 2},
    {'event_at': '2022-01-01 13:40:00+00:00', 'account_id': 'brian', 'subject': 'experimented', 'commit_count': 1},
    {'event_at': '2022-01-01 15:00:00+00:00', 'account_id': 'brian', 'subject': 'developed', 'commit_count': 1},
    {'event_at': '2022-01-01 15:10:00+00:00', 'account_id': 'brian', 'subject': 'wrote_code', 'commit_count': 4},
    {'event_at': '2022-01-01 15:20:00+00:00', 'account_id': 'brian', 'subject': 'developed', 'commit_count': 5},
    {'event_at': '2022-01-01 15:30:00+00:00', 'account_id': 'brian', 'subject': 'wrote_code', 'commit_count': 1},
    {'event_at': '2022-01-01 15:40:00+00:00', 'account_id': 'brian', 'subject': 'experimented', 'commit_count': 2},
    {'event_at': '2022-01-01 15:50:00+00:00', 'account_id': 'brian', 'subject': 'developed', 'commit_count': 1},
    {'event_at': '2022-01-01 16:00:00+00:00', 'account_id': 'brian', 'subject': 'wrote_code', 'commit_count': 2},
    {'event_at': '2022-01-01 16:10:00+00:00', 'account_id': 'brian', 'subject': 'built_model', 'commit_count': 1},
    {'event_at': '2022-01-01 16:20:00+00:00', 'account_id': 'brian', 'subject': 'built_model', 'commit_count': 1},
    {'event_at': '2022-01-01 16:30:00+00:00', 'account_id': 'brian', 'subject': 'experimented', 'commit_count': 3},
])


@data_source(training_data=df, keys='account_id', timestamp='event_at')
class Commit(TypedDict):
    event_at: datetime
    account_id: str
    subject: str
    commit_count: int


@feature(keys='account_id', data_source=Commit)
@freshness(target='1m', invalid_after='10m')
def subject(this_row: Commit, ctx: Context) -> str:
    return this_row['subject']


subject.replay()


@feature(keys='account_id', data_source=Commit)
@aggregation(function=AggregationFunction.DistinctCount, over='2h', granularity='10m')
def unique_tasks_over_2h(this_row: Commit, ctx: Context) -> str:
    return this_row['subject']


unique_tasks_over_2h.replay()


@feature(keys='account_id', data_source=Commit)
@aggregation(
    function=[AggregationFunction.Sum, AggregationFunction.Count, AggregationFunction.Max],
    over='30m', granularity='1m')
def commits_30m(this_row: Commit, ctx: Context) -> int:
    """sum/max/count of commits over 30 minutes"""

    return this_row['commit_count']


commits_30m.replay()


@feature(keys='account_id', data_source=Commit)
@freshness(target='1m', invalid_after='30m')
def commits_30m_greater_2(this_row: Commit, ctx: Context) -> bool:
    res, _ = ctx.get_feature('commits_30m+sum')
    return res > 2


commits_30m_greater_2.replay()


@model(
    keys=['account_id'],
    input_features=[
        'commits_30m+sum', commits_30m_greater_2
    ],
    input_labels=[],
    model_framework='sklearn',
)
@freshness(target='1h', invalid_after='100h')
def newest():
    # TODO: implement
    pass


print(manifests())

ret = newest.features_and_labels(since=pd.to_datetime('2019-12-04 00:00'), until=pd.to_datetime('2023-01-04 00:00'))
print(ret)
