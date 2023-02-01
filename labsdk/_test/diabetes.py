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

from ..raptor import Context, data_source, feature, freshness, model, TrainingContext

df = pd.read_csv('https://raw.githubusercontent.com/plotly/datasets/master/diabetes.csv')
df.insert(0, 'id', range(0, len(df)))

df.head()

# %%
# Data cleansing and filling missing values
df['Glucose'] = df['Glucose'].replace(0, df['Glucose'].mean())
df['BloodPressure'] = df['BloodPressure'].replace(0, df['BloodPressure'].mean())
df['SkinThickness'] = df['SkinThickness'].replace(0, df['SkinThickness'].mean())
df['Insulin'] = df['Insulin'].replace(0, df['Insulin'].mean())
df['BMI'] = df['BMI'].replace(0, df['BMI'].mean())
df['timestamp'] = pd.to_datetime(datetime.now())  # add timestamp


# %%

@data_source(training_data=df, keys=['id'], timestamp='timestamp')
class Diabetes(TypedDict):
    id: int
    Pregnancies: int
    Glucose: int
    BloodPressure: int
    SkinThickness: int
    Insulin: int
    BMI: float
    DiabetesPedigreeFunction: float
    Age: int
    Outcome: int


# %%

@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def pregnancies(this_row: Diabetes, ctx: Context) -> int:
    return this_row['Pregnancies']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def glucose(this_row: Diabetes, ctx: Context) -> int:
    return this_row['Glucose']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def blood_pressure(this_row: Diabetes, ctx: Context) -> int:
    return this_row['BloodPressure']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def skin_thickness(this_row: Diabetes, ctx: Context) -> int:
    return this_row['SkinThickness']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def insulin(this_row: Diabetes, ctx: Context) -> int:
    return this_row['Insulin']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def bmi(this_row: Diabetes, ctx: Context) -> float:
    return this_row['BMI']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def diabetes_pedigree_function(this_row: Diabetes, ctx: Context) -> float:
    return this_row['DiabetesPedigreeFunction']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def age(this_row: Diabetes, ctx: Context) -> int:
    return this_row['Age']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def outcome(this_row: Diabetes, ctx: Context) -> int:
    return this_row['Outcome']


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def bmi_class(this_row: Diabetes, ctx: Context) -> int:
    if this_row['BMI'] < 18.5:
        return -1  # Underweight
    elif 18.5 <= this_row['BMI'] <= 24.9:
        return 0  # Normal
    elif 25 <= this_row['BMI'] <= 29.9:
        return 1  # Overweight
    elif this_row['BMI'] >= 30:
        return 3  # Obesity


@feature(data_source=Diabetes, keys=['id'])
@freshness(target='1h', invalid_after='100h')
def insulin_class(this_row: Diabetes, ctx: Context) -> int:
    if this_row['Insulin'] < 16:
        return -1  # Low
    elif 16 <= this_row['Insulin'] <= 166:
        return 0  # Normal
    elif this_row['Insulin'] >= 166:
        return 1  # High


# %%

@model(
    keys=['id'],
    input_features=[
        pregnancies, glucose, blood_pressure, skin_thickness,
        insulin, bmi, diabetes_pedigree_function, age, bmi_class, insulin_class
    ],
    input_labels=[outcome],
    model_framework='sklearn',
    model_server='sagemaker-ack',
)
@freshness(target='1h', invalid_after='100h')
def diabetes_prediction_train(ctx: TrainingContext):
    from sklearn.model_selection import train_test_split
    from sklearn.ensemble import RandomForestClassifier

    df = ctx.features_and_labels()

    X = df[ctx.input_features]
    y = df[ctx.input_labels]

    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, stratify=y, random_state=1234)

    model = RandomForestClassifier(max_depth=2)
    model.fit(X_train, y_train.values.ravel())
    return model


mymodel = diabetes_prediction_train()

# %%

data = diabetes_prediction_train.features_and_labels()

from sklearn.model_selection import train_test_split
from sklearn.metrics import classification_report

x = data[diabetes_prediction_train.input_features]
y = data[diabetes_prediction_train.input_labels]
_, x_test, _, y_test = train_test_split(x, y, test_size=0.2, stratify=y, random_state=1234)

y_pred = mymodel.predict(x_test)
res = classification_report(y_pred, y_test.values.ravel())
print(res)
print('Accuracy:', mymodel.score(x_test, y_test.values.ravel()))

print('Test data')
with pd.option_context('display.max_rows', None, 'display.max_columns', None):
    print(pd.concat([x_test, y_test], axis=1).head(15))

print('test data as json')
for i in range(0, 10):
    print('row:', i)
    print(x_test.iloc[i].to_json())
    print('label:', y_test.values.ravel()[i])
    print('prediction:', mymodel.predict(x_test.iloc[i].to_frame().transpose()))

# Output
diabetes_prediction_train.export()
print('done')
