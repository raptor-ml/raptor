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
from typing import List, Callable

import pandas as pd


class TrainingContext:
    """
    Context of the model request.
    """

    keys: List[str] = []
    input_features: List[str] = []
    input_labels: List[str] = []
    _data_getter: Callable[[], pd.DataFrame] = None

    def __init__(self, keys: List[str], input_features: List[str], input_labels: List[str],
                 data_getter: Callable[[], pd.DataFrame]):
        self.keys = keys
        self.input_features = input_features
        self.input_labels = input_labels
        self._data_getter = data_getter

    def features_and_labels(self) -> pd.DataFrame:
        """
        Get the features and labels for the model
        """
        return self._data_getter()


